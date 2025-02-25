package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"log"
	"net/http"
	"os"
	"time"
)

func feltrinelliRepricer() {
	startTime := time.Now()

	ebayDataWithFeltrinelliBooks := getAllBooksOnEbay()

	bookPartialsChannel := make(chan BookPartial, 60)

	go scrapeRepricerBooksFeltrinelli(ebayDataWithFeltrinelliBooks, bookPartialsChannel)

	var booksToUpdate []BookToUpdate
	for bookPartial := range bookPartialsChannel {
		ebayDataWithFeltrinelliBook := ebayDataWithFeltrinelliBooks[bookPartial.ISBN]
		booksToUpdate = addBookToUpdateToSliceWithFeltrinelli(bookPartial, ebayDataWithFeltrinelliBook.FeltrinelliBook, booksToUpdate)
	}

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	bulkInsertBooksToUpdate(booksToUpdate, true)
	fmt.Println("Finished updating ", len(booksToUpdate), " books.")

	startPythonRepricer(booksToUpdate, true)
}

func addBookToUpdateToSliceWithFeltrinelli(bookInfo BookPartial, feltrinelliBook FeltrinelliBook, booksToUpdate []BookToUpdate) []BookToUpdate {
	bookToUpdate := BookToUpdate{bookInfo.ISBN, bookInfo.Price, false, bookInfo.Available, false}
	if feltrinelliBook.Availability != bookInfo.Available {
		bookToUpdate.AvailabilityChanged = true
		fmt.Println("Availability changed to ", bookInfo.Available, " for ", bookInfo.ISBN)
	}
	if feltrinelliBook.Price != bookInfo.Price {
		bookToUpdate.PriceChanged = true
		fmt.Println("Price changed to ", bookInfo.Price, " for ", bookInfo.ISBN)
	}

	if bookToUpdate.AvailabilityChanged || bookToUpdate.PriceChanged {
		booksToUpdate = append(booksToUpdate, bookToUpdate)
	}
	return booksToUpdate
}

func scrapeRepricerBooksFeltrinelli(ebayDataWithFeltrinelliBooks map[string]EbayDataWithFeltrinelliBook, bookPartialChannel chan<- BookPartial) {
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

	semaphore := NewSemaphore(50)
	booksMap := make(map[string]BookPartial)

	c.OnHTML("pdp-physical-buy-info", func(e *colly.HTMLElement) {
		if e.Attr(":is-ebook") == "true" || e.Attr(":is-marketplace") == "true" {
			log.Fatalf("Book during repricing is ebook or from marketplace")
			return
		}
		EAN := getEANFromPath(e.Request.URL.Path)
		var inventoryJSON InventoryJSON
		if unmarshalJSON([]byte(e.Attr(":inventory")), &inventoryJSON) != nil {
			return
		}
		var availabilityJSON AvailabilityJSON
		if unmarshalJSON([]byte(e.Attr(":availability")), &availabilityJSON) != nil {
			return
		}

		price, _ := formatNumber(inventoryJSON.Price)
		booksMap[EAN] = BookPartial{
			Price:     price,
			Available: availabilityJSON.Text,
		}
	})

	c.OnScraped(func(response *colly.Response) {
		semaphore.Release()
		EAN := getEANFromPath(response.Request.URL.Path)
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
