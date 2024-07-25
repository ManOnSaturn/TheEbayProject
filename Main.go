package main

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/context"
	"os"
	"sync"
	"time"
)

type BookPartial struct {
	ISBN      string
	Published bool
	Price     string
	Available bool
	URL       string
}

type DBBook struct {
	ISBN      string
	Published bool
	Available bool
	Price     string
	URL       string
}

type BookFull struct {
	ISBN      string
	Published bool
	Title     string
	Available bool
	Price     string
	URL       string
	ImageURL  string
	Author    string
	Category  string
	Variant   string
	Editor    string
	Language  string
}

func main() {
	startTime := time.Now()

	if os.Args[1] == "--fullScrape" {
		fullScraping()
	}
	if os.Args[1] == "--repricer" {
		repricer()
	}

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}

func repricer() {}

func fullScraping() {
	startTime := time.Now()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	scrapingFinishedChannel := make(chan bool)

	go func() {
		select {
		case <-scrapingFinishedChannel:
			cancel()
		case <-ctx.Done():
			panic("Scraping timed out")
		}
	}()

	booksChannel := make(chan BookFull, 60)

	go getBooks(booksChannel)

	bookSet := make(map[BookFull]bool, 750000)

	for bookInfo := range booksChannel {
		bookSet[bookInfo] = true
	}

	scrapingFinishedChannel <- true
	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	client := connectToMongo()
	defer func(client *mongo.Client) {
		err := client.Disconnect(context.TODO())
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error while disconnecting client.")
			if err != nil {
				fmt.Println(err)
			}
		}
	}(client)

	booksCollection := client.Database("Mondadori").Collection("Books")
	booksToAdd, booksToUpdate := getBooksToAddAndUpdate(booksCollection, bookSet)

	var mongoDBOperationsWG sync.WaitGroup

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		insertBooks(booksCollection, booksToAdd)
		fmt.Println("Finished inserting new books in DB.")
	}()

	mongoDBOperationsWG.Wait()
	bulkUpdateBooks(booksCollection, booksToUpdate)
	fmt.Println("Finished updating books in DB.")
}

func getBooksToAddAndUpdate(booksCollection *mongo.Collection, scrapedBooksSet map[BookFull]bool) ([]*BookFull, []*BookFull) {
	dbBooks := make(map[string]BookFull, 750000)
	getAllDBBooks(booksCollection, dbBooks)
	var booksToAdd []*BookFull
	var booksToUpdate []*BookFull

	fmt.Println("Creating lists of books to create and books to update.")
	startTime := time.Now()
	for bookInfo := range scrapedBooksSet {
		bookFromDB, ok := dbBooks[bookInfo.ISBN]
		if !ok {
			// If the book ISBN from the scraped books set is not in the DB, then add the book to the DB
			booksToAdd = append(booksToAdd, &bookInfo)
		} else if !bookInfo.Published && bookFromDB != bookInfo {
			// Else if the scraped book is in the DB, it's unpublished from us, and it has some differences, update it
			booksToUpdate = append(booksToUpdate, &bookInfo)
		}
	}

	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")
	fmt.Println("Finished creating lists of books to add and update in DB in", time.Since(startTime).Seconds(), "seconds.")
	return booksToAdd, booksToUpdate
}
