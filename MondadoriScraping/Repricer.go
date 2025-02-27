package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func Repricer() {
	startTime := time.Now()

	dbPublishedBooks := MongoDBInteractions.GetAllPublishedBooks()

	booksChannel := make(chan DataTypes.BookPartial, 60)

	go scrapeRepricerBooks(dbPublishedBooks, booksChannel)

	var booksToUpdate []DataTypes.BookToUpdate
	for bookInfo := range booksChannel {
		dbBook := dbPublishedBooks[bookInfo.ISBN]
		booksToUpdate = addBookToUpdateToSlice(bookInfo, dbBook, booksToUpdate)
	}

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	MongoDBInteractions.BulkInsertBooksToUpdate(booksToUpdate, false)
	fmt.Println("Finished updating ", len(booksToUpdate), " books.")

	StartPythonRepricer(booksToUpdate, false)
}

func StartPythonRepricer(booksToUpdate []DataTypes.BookToUpdate, isFeltrinelli bool) {
	// Start python repricer if there is any book to update.
	if len(booksToUpdate) > 0 {
		var repricerString string
		if isFeltrinelli {
			repricerString = "--feltrinelliRepricer"
		} else {
			repricerString = "--repricer"
		}
		cmd := exec.Command("/bin/bash", "/home/mattia/repricer/start_repricer.sh", repricerString)
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

func addBookToUpdateToSlice(bookInfo DataTypes.BookPartial, dbBook DataTypes.BookFull, booksToUpdate []DataTypes.BookToUpdate) []DataTypes.BookToUpdate {
	bookToUpdate := DataTypes.BookToUpdate{bookInfo.ISBN, bookInfo.Price, false, bookInfo.Available, false}
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
	return booksToUpdate
}
