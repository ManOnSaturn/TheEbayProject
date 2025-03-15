package FeltrinelliScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"time"
)

func Repricer() {
	startTime := time.Now()

	ebayDataWithFeltrinelliBooks := MongoDBInteractions.GetAllFeltrinelliBooksOnEbay()

	bookPartialsChannel := make(chan DataTypes.BookPartial, 60)

	go scrapeRepricerBooks(ebayDataWithFeltrinelliBooks, bookPartialsChannel)

	updateModels := make([]mongo.WriteModel, 0)
	for bookPartial := range bookPartialsChannel {
		updateModels = append(updateModels, MongoDBInteractions.BuildFeltrinelliPriceOrAvailabilityUpdateModel(bookPartial))
	}

	MongoDBInteractions.UpdateFeltrinelliPriceOrAvailability(updateModels)

	fmt.Println("Finished Feltrinelli's repricer in", time.Since(startTime).Seconds(), "seconds.")
}

func scrapeRepricerBooks(ebayDataWithFeltrinelliBooks map[string]DataTypes.EbayDataWithFeltrinelliBook, bookPartialChannel chan<- DataTypes.BookPartial) {
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")), colly.Async(true))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			//reqURL := req.URL.String()
			//originalReqURL := via[len(via)-1].URL.String()
			//if areURLsForSameBook(originalReqURL, reqURL) {
			//	setNewURLAndIsBook(originalReqURL, reqURL, true)
			//	return nil
			//}
			//setProductIsBook(originalReqURL, false)
			return fmt.Errorf("redirects are not allowed")
		},
	})

	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 10,
	})
	if err != nil {
		panic(err)
	}
	c.SetRequestTimeout(30 * time.Second)

	semaphore := DataTypes.NewSemaphore(50)
	booksMap := make(map[string]DataTypes.BookPartial)

	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		if e.Attr(":is-ebook") == "true" || e.Attr(":is-marketplace") == "true" {
			log.Fatalf("Book during repricing is ebook or from marketplace. URL:", e.Request.URL.String())
			return
		}
		EAN := GetEANFromPath(e.Request.URL.Path)
		var inventoryJSON DataTypes.InventoryJSON
		if UnmarshalJSON([]byte(e.Attr(":inventory")), &inventoryJSON) != nil {
			return
		}
		var availabilityJSON DataTypes.AvailabilityJSON
		if UnmarshalJSON([]byte(e.Attr(":availability")), &availabilityJSON) != nil {
			return
		}

		price, _ := MongoDBInteractions.FormatNumberIntoString(inventoryJSON.Price)
		booksMap[EAN] = DataTypes.BookPartial{
			Price:     price,
			Available: availabilityJSON.Text,
		}
	})

	c.OnScraped(func(response *colly.Response) {
		semaphore.Release()
		EAN := GetEANFromPath(response.Request.URL.Path)
		if fullBook, ok := booksMap[EAN]; !ok {
			log.Fatalf("Failed to retrieve EAN in booksMap: %s", EAN)
		} else {
			bookPartialChannel <- fullBook
			delete(booksMap, EAN)
		}
	})

	c.OnError(func(response *colly.Response, err error) {
		semaphore.Release()
		_, err2 := fmt.Fprintf(os.Stderr, "error for request:%s, %v\n", response.Request.URL, err)
		if err2 != nil {
			panic(err2)
		}
	})

	for _, ebayDataWithFeltrinelliBook := range ebayDataWithFeltrinelliBooks {
		semaphore.Acquire()
		err := c.Visit(ebayDataWithFeltrinelliBook.FeltrinelliBook.URL)
		if err != nil {
			log.Fatalf("Failed to visit:%s", err)
		}
	}
	c.Wait()
	close(bookPartialChannel)
}
