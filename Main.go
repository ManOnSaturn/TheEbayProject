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
}

type DBBook struct {
	ISBN      string
	Published bool
	Available bool
	Price     string
	Found     bool
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

	booksChannel := make(chan BookFull, 500)
	var wg sync.WaitGroup

	go func() {
		defer wg.Done()
		wg.Add(1)
		getBooks(booksChannel)
		fmt.Println("[DEBUG] Went after getBooks.")
	}()

	go func() {
		wg.Wait()
		close(booksChannel)
	}()

	bookSet := make(map[BookFull]bool, 750000)

Loop:
	for {
		select {
		// Wait for all URLs to be gathered before closing the channel
		case bookInfo, ok := <-booksChannel:
			if !ok {
				break Loop
			}
			bookSet[bookInfo] = true
		}
	}

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

	booksToUpdateCollection := client.Database("Mondadori").Collection("BooksToUpdate")

	var booksToBulkUpdate []*BookPartial
	var booksToBulkInsert []*BookPartial
	for _, bookToUpdate := range booksToUpdate {
		if bookToUpdate.Published {
			// If it's published, it needs handling on the python side.
			booksToBulkInsert = append(booksToBulkInsert, &bookToUpdate)
		} else {
			// Else, we just put the updated info in the main collection.
			booksToBulkUpdate = append(booksToBulkUpdate, &bookToUpdate)
		}
	}

	mongoDBOperationsWG.Wait()
	bulkUpdateBooks(booksCollection, booksToBulkUpdate)
	fmt.Println("Finished updating books in DB.")
	bulkInsertBooksToUpdate(booksToUpdateCollection, booksToBulkInsert)
	fmt.Println("Finished inserting books to update in DB.")

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}

func getBooksToAddAndUpdate(booksCollection *mongo.Collection, scrapedBooksSet map[BookFull]bool) ([]*BookFull, []BookPartial) {
	dbBooks := make(map[string]DBBook, 750000)
	getAllDBBooks(booksCollection, dbBooks)
	var booksToAdd []*BookFull
	var booksToUpdate []BookPartial

	fmt.Println("Creating lists of books to create and books to update.")
	for bookInfo := range scrapedBooksSet {
		bookFromDB, ok := dbBooks[bookInfo.ISBN]
		if !ok {
			booksToAdd = append(booksToAdd, &bookInfo)
		} else {
			bookFromDB.Found = true
			dbBooks[bookFromDB.ISBN] = bookFromDB
			// Add book to the list of books to update if either availability or price change.
			if bookFromDB.Available != bookInfo.Available || bookFromDB.Price != bookInfo.Price {
				booksToUpdate = append(booksToUpdate, BookPartial{ISBN: bookFromDB.ISBN, Published: bookFromDB.Published, Price: bookInfo.Price, Available: bookInfo.Available})
			}
		}
	}
	// If a database book has not been touched when going through all the books previously found on Mondadori,
	// it means we have lost track of it, and we mark it as unavailable.
	for _, book := range dbBooks {
		if !book.Found {
			booksToUpdate = append(booksToUpdate, BookPartial{ISBN: book.ISBN, Price: book.Price, Available: false})
		}
	}

	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")
	return booksToAdd, booksToUpdate
}
