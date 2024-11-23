package main

import (
	"fmt"
	"golang.org/x/net/context"
	"os"
	"os/exec"
	"time"
)

func repricer() {
	startTime := time.Now()

	dbPublishedBooks := make(map[string]BookFull, 5000)
	getAllPublishedBooks(dbPublishedBooks)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	scrapingFinishedChannel := make(chan bool)

	go func() {
		select {
		case <-scrapingFinishedChannel:
			cancel()
		case <-ctx.Done():
			panic("Repricing timed out")
		}
	}()

	booksChannel := make(chan BookPartial, 60)

	go scrapeRepricerBooks(dbPublishedBooks, booksChannel)

	var booksToUpdate []BookToUpdate
	for bookInfo := range booksChannel {
		dbBook := dbPublishedBooks[bookInfo.ISBN]
		bookToUpdate := BookToUpdate{bookInfo.ISBN, bookInfo.Price, false, bookInfo.Available, false}
		if dbBook.Available != bookInfo.Available {
			bookToUpdate.AvailabilityChanged = true
			fmt.Println("Availability changed to ", bookInfo.Available, " for ", bookInfo.ISBN)
		}
		if dbBook.Price != bookInfo.Price {
			bookToUpdate.PriceChanged = true
			fmt.Println("Price changed to ", bookInfo.Price, " for ", bookInfo.ISBN)
		}

		if bookToUpdate.AvailabilityChanged || bookToUpdate.PriceChanged {
			booksToUpdate = append(booksToUpdate, bookToUpdate)
		}
	}

	scrapingFinishedChannel <- true
	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	bulkInsertBooksToUpdate(booksToUpdate)
	fmt.Println("Finished updating ", len(booksToUpdate), " books.")

	// Start python repricer if there is any book to update.
	if len(booksToUpdate) > 0 {
		cmd := exec.Command("/bin/bash", "/home/mattia/repricer/start_repricer.sh", "--repricer")
		output, err := cmd.CombinedOutput()
		if err != nil {
			_, err := fmt.Fprintf(os.Stderr, "Error in starting repricer script from GoLang to Python: %s", err)
			if err != nil {
				return
			}
			return
		}
		fmt.Printf("%s\n", output)
	}
}
