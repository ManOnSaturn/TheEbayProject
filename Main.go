package main

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
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

	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds")

	client, err := connectToMongo("mongodb://localhost:27017/")
	if err != nil {
		fmt.Println("Error occurred while trying to connect to MongoDB")
		return
	}

	booksCollection := client.Database("Mondadori").Collection("Books")
	fmt.Println("Connected to MongoDB!")

	booksToAdd, booksToUpdate := getBooksToAddAndUpdate(booksCollection, bookSet)

	var mongoDBOperationsWG sync.WaitGroup

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		insertBooks(booksCollection, booksToAdd)
		fmt.Println("Finished inserting books in Books DB")
	}()

	mongoDBOperationsWG.Wait()

	booksToUpdateCollection := client.Database("Mondadori").Collection("BooksToUpdate")

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		for _, bookToUpdate := range booksToUpdate {
			if bookToUpdate.Published {
				// If it's published, it needs handling on the python side.
				insertBookToUpdate(booksToUpdateCollection, bookToUpdate)
			} else {
				// Else, we just put the updated info in the main collection.
				updateBookToUpdate(booksCollection, bookToUpdate)
			}
		}
		fmt.Println("Finished inserting books in BooksToUpdate DB")
	}()

	mongoDBOperationsWG.Wait()

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds")
}

func getBooksToAddAndUpdate(booksCollection *mongo.Collection, booksMap map[BookFull]bool) ([]BookFull, []BookPartial) {
	dbBooks := make(map[string]DBBook, 750000)
	getAllDB(booksCollection, dbBooks)
	var booksToAdd []BookFull
	var booksToUpdate []BookPartial

	fmt.Println("Creating lists of books to create and books to update")
	for bookInfo := range booksMap {
		bookFromDB, ok := dbBooks[bookInfo.ISBN]
		if !ok {
			booksToAdd = append(booksToAdd, bookInfo)
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
