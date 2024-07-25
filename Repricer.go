package main

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/context"
	"os"
	"os/exec"
	"time"
)

func Repricer() {
	startTime := time.Now()

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
	dbPublishedBooks := make(map[string]BookFull, 5000)
	getAllPublishedBooks(booksCollection, dbPublishedBooks)

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

	booksChannel := make(chan BookPartial, 60)

	go scrapeRepricerBooks(dbPublishedBooks, booksChannel)

	var booksToUpdate []*BookPartial
	for bookInfo := range booksChannel {
		dbBook := dbPublishedBooks[bookInfo.ISBN]
		if dbBook.Price != bookInfo.Price || bookInfo.Available == false {
			booksToUpdate = append(booksToUpdate, &bookInfo)
		}
	}

	scrapingFinishedChannel <- true
	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds.")

	booksToUpdateCollection := client.Database("Mondadori").Collection("BooksToUpdate")

	bulkInsertBooksToUpdate(booksToUpdateCollection, booksToUpdate)
	fmt.Println("Finished updating ", len(booksToUpdate), " books.")
	cmd := exec.Command("/bin/bash", "/home/mattia/ebay/repricer/start_repricer.sh")

	output, err := cmd.CombinedOutput()
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Error executing script: %s", err)
		if err != nil {
			return
		}
		return
	}
	fmt.Printf("%s\n", output)

}
