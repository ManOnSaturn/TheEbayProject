package MondadoriScraping

import (
	"Scraper/DataTypes"
	"Scraper/MongoDBInteractions"
	"fmt"
	"sync"
	"time"
)

func FullScrape() {
	startTime := time.Now()

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

	booksChannel := make(chan DataTypes.BookFull, 60)

	go getBooks(booksChannel)

	bookSet := make(map[DataTypes.BookFull]bool, 750000)

	for bookInfo := range booksChannel {
		bookSet[bookInfo] = true
	}

	//scrapingFinishedChannel <- true
	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	booksToAdd, booksToUpdate := getBooksToAddAndUpdate(bookSet)

	var mongoDBOperationsWG sync.WaitGroup

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		MongoDBInteractions.InsertBooks(booksToAdd)
		fmt.Println("Finished inserting new books in DB.")
	}()

	mongoDBOperationsWG.Wait()
	MongoDBInteractions.BulkUpdateBooks(booksToUpdate)
	fmt.Println("Finished updating books in DB.")
}

func getBooksToAddAndUpdate(scrapedBooksSet map[DataTypes.BookFull]bool) ([]*DataTypes.BookFull, []*DataTypes.BookFull) {
	dbBooks := make(map[string]DataTypes.BookFull, 750000)
	MongoDBInteractions.GetAllDBBooks(dbBooks)
	var booksToAdd []*DataTypes.BookFull
	var booksToUpdate []*DataTypes.BookFull

	fmt.Println("Creating lists of books to create and books to update.")
	startTime := time.Now()
	for bookInfo := range scrapedBooksSet {
		bookFromDB, ok := dbBooks[bookInfo.ISBN]
		if !ok {
			// If the book ISBN from the scraped books set is not in the DB, then add the book to the DB
			booksToAdd = append(booksToAdd, &bookInfo)
		} else if !bookFromDB.Published && bookFromDB != bookInfo {
			// Else if the scraped book is in the DB, it's unpublished from us, and it has some differences, update it
			booksToUpdate = append(booksToUpdate, &bookInfo)
		}
	}

	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")
	fmt.Println("Finished creating lists of books to add and update in DB in", time.Since(startTime).Seconds(), "seconds.")
	return booksToAdd, booksToUpdate
}
