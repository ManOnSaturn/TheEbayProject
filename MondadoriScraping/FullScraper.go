package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func FullScrape() {
	scrapeXMLs()

	urlsChan := make(chan string, 100)
	booksChan := make(chan *DataTypes.MondadoriBook, 100)
	proxies := Proxy.GetProxies()

	wg := sync.WaitGroup{}
	wg.Add(len(proxies))

	fakeChrome := getChromeClient()
	for _, proxy := range proxies {
		go func(proxy string) {
			defer wg.Done()
			scrapeBooks(urlsChan, booksChan, proxy, fakeChrome)
		}(proxy)
	}

	go MongoDBInteractions.GetAllMondadoriURLs(urlsChan)

	go func() {
		wg.Wait()
		close(booksChan)
	}()
	var models []mongo.WriteModel
	var models2 []mongo.WriteModel
	for book := range booksChan {
		models = append(models, MongoDBInteractions.CreateUpsertModelFromMondadoriBook(*book))
		models2 = append(models2, MongoDBInteractions.CreateUpsertModelFromMondadoriBookForProducts(*book))
		if len(models) == 1000 {
			MongoDBInteractions.UpsertMondadoriBooks(models)
			models = make([]mongo.WriteModel, 0)
			MongoDBInteractions.UpsertMondadoriISBNInProducts(models2)
			models2 = make([]mongo.WriteModel, 0)
		}
	}

	if len(models) > 0 {
		MongoDBInteractions.UpsertMondadoriBooks(models)
		MongoDBInteractions.UpsertMondadoriISBNInProducts(models2)
	}
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

	for url := range urlsChan {
		err := c.Visit(url)
		if err != nil {
			panic(err)
		}
	}

	c.Wait()
}

var chromeClient *req.Client

func getChromeClient() *req.Client {
	if chromeClient == nil {
		chromeClient = req.DefaultClient().ImpersonateChrome().DisableKeepAlives()
	}
	return chromeClient
}
