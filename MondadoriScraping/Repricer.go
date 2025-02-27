package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"Scraper/PythonInteractions"
	"fmt"
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

	PythonInteractions.StartPythonRepricer(booksToUpdate, false)
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
