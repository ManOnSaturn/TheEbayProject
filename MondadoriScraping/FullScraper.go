package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/Proxy"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func FullScrape() {
	//startTime := time.Now()

	//// Create a context with timeout
	//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	//defer cancel()
	//scrapingFinishedChannel := make(chan bool)
	//
	//go func() {
	//	select {
	//	case <-scrapingFinishedChannel:
	//		cancel()
	//	case <-ctx.Done():
	//		panic("Scraping timed out")
	//	}
	//}()

	//booksChannel := make(chan DataTypes.MondadoriBook, 60)
	//
	//go getBooks(booksChannel)
	//
	//bookSet := make(map[DataTypes.MondadoriBook]bool, 750000)
	//
	//for bookInfo := range booksChannel {
	//	bookSet[bookInfo] = true
	//}
	//
	////scrapingFinishedChannel <- true
	//fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")
	//
	//booksToAdd, booksToUpdate := getBooksToAddAndUpdate(bookSet)
	//
	//var mongoDBOperationsWG sync.WaitGroup
	//
	//mongoDBOperationsWG.Add(1)
	//go func() {
	//	defer mongoDBOperationsWG.Done()
	//	MongoDBInteractions.InsertBooks(booksToAdd)
	//	fmt.Println("Finished inserting new books in DB.")
	//}()
	//
	//mongoDBOperationsWG.Wait()
	//MongoDBInteractions.BulkUpdateBooks(booksToUpdate)
	//fmt.Println("Finished updating books in DB.")
}

func NewFullScrape() {
	//scrapeXMLs()

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

	urlsChan <- "https://www.mondadoristore.it/MillenniuM-Ediz-speciale-2024-Vol-86-Fatela-finita-L-Ucraina-Gaza-e-le-altre-foto-e-parole-na/eai979128198504/"
	urlsChan <- "https://www.mondadoristore.it/Filologia-germanica-Lingue-Nicoletta-Francovich-Onesti/eai978884302315/"
	urlsChan <- "https://www.mondadoristore.it/Modernist-bread-at-home-Ediz-italiana/eai979898871311/"
	close(urlsChan)
	//go MongoDBInteractions.GetAllMondadoriURLs(urlsChan)

	go func() {
		wg.Wait()
		close(booksChan)
	}()

	for book := range booksChan {
		fmt.Println(book)
		MongoDBInteractions.UpsertMondadoriBook(*book)
		MongoDBInteractions.UpsertMondadoriISBNInProducts(*book)
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

//func getBooksToAddAndUpdate(scrapedBooksSet map[DataTypes.MondadoriBook]bool) ([]*DataTypes.MondadoriBook, []*DataTypes.MondadoriBook) {
//	dbBooks := make(map[string]DataTypes.MondadoriBook, 750000)
//	MongoDBInteractions.GetAllDBBooks(dbBooks)
//	var booksToAdd []*DataTypes.MondadoriBook
//	var booksToUpdate []*DataTypes.MondadoriBook
//
//	fmt.Println("Creating lists of books to create and books to update.")
//	startTime := time.Now()
//	for bookInfo := range scrapedBooksSet {
//		bookFromDB, ok := dbBooks[bookInfo.ISBN]
//		if !ok {
//			// If the book ISBN from the scraped books set is not in the DB, then add the book to the DB
//			booksToAdd = append(booksToAdd, &bookInfo)
//		} else if !bookFromDB.Published && bookFromDB != bookInfo {
//			// Else if the scraped book is in the DB, it's unpublished from us, and it has some differences, update it
//			booksToUpdate = append(booksToUpdate, &bookInfo)
//		}
//	}
//
//	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")
//	fmt.Println("Finished creating lists of books to add and update in DB in", time.Since(startTime).Seconds(), "seconds.")
//	return booksToAdd, booksToUpdate
//}
