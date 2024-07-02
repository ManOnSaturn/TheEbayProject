package main

import (
	"fmt"
	"sync"
	"time"
)

type BookPartial struct {
	ISBN      string
	Price     string
	Available bool
}

type BookFull struct {
	ISBN      string
	URL       string
	Price     string
	Available bool
	Title     string
	Author    string
	Category  string
	Language  string
	Variant   string
	Editor    string
	ImageURL  string
}

func main() {
	startTime := time.Now()

	bookInfoChannel := make(chan BookFull, 500)
	var wg sync.WaitGroup

	go func() {
		defer wg.Done()
		wg.Add(1)
		getBooks(bookInfoChannel)
	}()

	go func() {
		wg.Wait()
		close(bookInfoChannel)
	}()

	bookInfosMap := make(map[BookFull]bool, 750000)

Loop:
	for {
		select {
		// Wait for all URLs to be gathered before closing the channel
		case bookInfo, ok := <-bookInfoChannel:
			if !ok {
				break Loop
			}
			bookInfosMap[bookInfo] = true
		}
	}

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds")

	client, err := connectToMongo("mongodb://localhost:27017/")
	booksCollection := client.Database("Mondadori").Collection("Books2")

	dbBooks := make(map[string]BookPartial, 750000)
	getAllDB(booksCollection, dbBooks)
	var booksToAdd []BookFull
	var booksToUpdate []BookPartial

	fmt.Println("Creating lists of books to create and books to update")
	for newBookInfo := range bookInfosMap {
		bookFromDB, ok := dbBooks[newBookInfo.ISBN]
		if !ok {
			booksToAdd = append(booksToAdd, newBookInfo)
		} else {
			// Add book to the list of books to update if either availability or price change.
			if bookFromDB.Available != newBookInfo.Available || bookFromDB.Price != newBookInfo.Price {
				booksToUpdate = append(booksToUpdate, BookPartial{ISBN: bookFromDB.ISBN, Price: newBookInfo.Price, Available: newBookInfo.Available})
			}
		}
	}
	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")

	if err != nil {
		fmt.Println("Error occurred while trying to connect to MongoDB")
		return
	}

	var mongoDBOperationsWG sync.WaitGroup

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		for _, bookToAdd := range booksToAdd {
			insertBook(booksCollection, bookToAdd)
		}
		fmt.Println("Finished inserting books in Books DB")
	}()

	mongoDBOperationsWG.Wait()

	booksToUpdateCollection := client.Database("Mondadori").Collection("BooksToUpdate")

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		for _, bookToUpdate := range booksToUpdate {
			insertBookToUpdate(booksToUpdateCollection, bookToUpdate)
		}
		fmt.Println("Finished inserting books in BooksToUpdate DB")
	}()

	mongoDBOperationsWG.Wait()

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds")
}
