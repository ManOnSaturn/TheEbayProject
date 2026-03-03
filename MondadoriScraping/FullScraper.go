package MondadoriScraping

import (
	"Scraper/ChromeClient"
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

func FullScrape() {
	scrapeXMLs()

	urlsChan := make(chan string, 100)
	booksChan := make(chan *DataTypes.MondadoriBook, 1000)
	proxies := Proxy.GetProxies()

	wg := sync.WaitGroup{}
	wg.Add(len(proxies))

	fakeChrome := ChromeClient.GetChromeClient()
	for _, proxy := range proxies {
		go func(proxy string) {
			defer wg.Done()
			scrapeBooks(urlsChan, booksChan, proxy, fakeChrome)
			fmt.Println("Finished with proxy", proxy)
		}(proxy)
	}

	go MongoDBInteractions.GetAllMondadoriURLs(urlsChan)

	go func() {
		wg.Wait()
		close(booksChan)
	}()

	var booksAndProductsUpserted int
	var models []mongo.WriteModel
	var models2 []mongo.WriteModel
	for book := range booksChan {
		models = append(models, MongoDBInteractions.CreateUpsertModelFromMondadoriBook(*book))
		models2 = append(models2, MongoDBInteractions.CreateUpsertModelFromMondadoriBookForProducts(*book))
		if len(models) == 1000 {
			MongoDBInteractions.UpsertMondadoriBooksAndProducts(models, models2)
			models = make([]mongo.WriteModel, 0)
			models2 = make([]mongo.WriteModel, 0)
			booksAndProductsUpserted += len(models)
			fmt.Println("Upserted ", booksAndProductsUpserted, " books and products.")
		}
	}

	if len(models) > 0 {
		MongoDBInteractions.UpsertMondadoriBooksAndProducts(models, models2)
		booksAndProductsUpserted += len(models)
		fmt.Println("Upserted ", booksAndProductsUpserted, " books and products.")
	}
	// In the future, we might want to check whether there are no new mondadori categories in the database, which we don't know of
	//with something like checkNoNewMondadoriCategory()
}

func scrapeBooks(urlsChan <-chan string, booksChan chan<- *DataTypes.MondadoriBook, proxy string, fakeChrome *req.Client) {
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
	})
	_ = c.Limit(&colly.LimitRule{DomainGlob: "*", Delay: 5 * time.Second})
	_ = c.SetProxy(proxy)

	var book *DataTypes.MondadoriBook
	c.OnResponse(func(response *colly.Response) {
		book = new(DataTypes.MondadoriBook)
	})

	c.OnHTML("a.link.nti-author", func(element *colly.HTMLElement) {
		book.Author = element.Text
	})

	c.OnHTML("div.intestation.intestation-new", func(element *colly.HTMLElement) {
		book.Title = element.ChildText("h1")
	})

	c.OnHTML("body", func(element *colly.HTMLElement) {
		book.Price = element.ChildAttr("span.new-price.new-detail-price", "content")
		book.Available = element.ChildText("span.big.lightGreen strong")
	})

	c.OnHTML("p.text[itemprop=description]", func(element *colly.HTMLElement) {
		book.Description = element.Text
	})

	c.OnHTML("div.product-details", func(element *colly.HTMLElement) {
		element.ForEach("p.text.text-full", func(i int, element2 *colly.HTMLElement) {
			intestation := element2.ChildText(".intestation")
			value := element2.ChildText(".value")

			// Clean up the text
			intestation = strings.TrimSpace(intestation)
			value = strings.TrimSpace(value)

			if intestation == "Generi" {
				value = strings.ReplaceAll(value, "\n", "")
				value = strings.ReplaceAll(value, "\t", "")
				value = strings.ReplaceAll(value, "» ", ">")
				book.Categories = strings.Split(value, ",")
			} else if intestation == "Editore" {
				book.Editor = value
			} else if intestation == "Formato" {
				book.Variant = value
			} else if intestation == "Pubblicato" {
				book.PublishedFrom = value
			} else if intestation == "Pagine" {
				book.Pages, _ = strconv.Atoi(value)
			} else if intestation == "Lingua" {
				book.Language = value
			} else if intestation == "Isbn o codice id" {
				book.ISBN = value
			} else if intestation == "Collana" {
				book.Series = value
			}
		})
	})

	c.OnHTML("div#galleria", func(element *colly.HTMLElement) {
		book.ImageURL = "https://mondadoristore.it" + element.ChildAttr("img", "src")
	})

	c.OnScraped(func(response *colly.Response) {
		url := response.Request.URL.String()
		book.URL = url
		booksChan <- book
	})

	errorsNumber := 0
	for url := range urlsChan {
		err := c.Visit(url)
		if err != nil {
			errorsNumber++
			_, err = fmt.Fprintf(os.Stderr, "Error during scrapeBooks with url %s: %v. Proxy %s has failed %d times.\n", url, err, proxy, errorsNumber)
			if err != nil {
				panic(err)
			}
			if errorsNumber > 100 {
				_, err = fmt.Fprintf(os.Stderr, "The same proxy(%s) failed 100 times.\n", proxy)
				if err != nil {
					panic(err)
				}
				return
			}
		}
	}

	c.Wait()
}
