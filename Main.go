package main

import (
	"Scraper/AmazonScraping"
	"Scraper/DataTypes"
	"Scraper/EbayBookBuilder"
	"Scraper/FeltrinelliScraping"
	"Scraper/MondadoriScraping"
	"Scraper/MongoDBInteractions"
	"Scraper/PythonInteractions"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)
import _ "net/http/pprof"

func main() {
	startTime := time.Now()
	MongoDBInteractions.ConnectToMongo()
	defer MongoDBInteractions.DisconnectFromMongo()

	if len(os.Args) > 1 && os.Args[1] == "--scrapeBestsellers" {
		AmazonScraping.ScrapeBestsellers()
	}

	if len(os.Args) > 1 && os.Args[1] == "--mondadoriRepricer" {
		MondadoriScraping.Repricer()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeFeltrinelli" {
		go func() {
			log.Println(http.ListenAndServe("0.0.0.0:8888", nil))
			panic("what")
		}()
		FeltrinelliScraping.FullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeMondadori" {
		MondadoriScraping.FullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--feltrinelliRepricer" {
		FeltrinelliScraping.Repricer()
	}

	//MongoDBInteractions.Reschema()
	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}

func reprice() {
	var wg sync.WaitGroup
	wg.Add(1)
	//wg.Add(2)
	//go func() {
	//	defer wg.Done()
	//	FeltrinelliScraping.Repricer()
	//}()
	go func() {
		defer wg.Done()
		MondadoriScraping.Repricer()
	}()
	wg.Wait()

	mondadoriBooks := MongoDBInteractions.GetAllMondadoriBooksOnEbay()
	isbns := make([]string, 0)
	for _, mondadoriBook := range mondadoriBooks {
		isbns = append(isbns, mondadoriBook.MondadoriBook.ISBN)
	}
	ebayBooks := EbayBookBuilder.BuildEbayBooks(isbns)
	oldEbayBooks := MongoDBInteractions.GetAllEbayBooks()
	var updateModels []mongo.WriteModel
	for _, oldEbayBook := range oldEbayBooks {
		newEbayBook := ebayBooks[oldEbayBook.ISBN]
		if !newEbayBook.Equals(oldEbayBook) {
			MongoDBInteractions.AddBookToUpdate(oldEbayBook.ISBN)
			updateModels = append(updateModels, MongoDBInteractions.CreateUpsertModelForEbayBooks(newEbayBook))
		}
	}
	MongoDBInteractions.UpsertEbayBooks(updateModels)
	PythonInteractions.StartPythonRepricer(len(updateModels), false)
}

func Reschema() {
	cursor, _ := MongoDBInteractions.MondadoriBooksCollection.Find(context.TODO(), bson.M{"ListingId": bson.M{"$exists": true}})
	var ebayDataModels []mongo.WriteModel
	var ebayBooksModels []mongo.WriteModel
	for cursor.Next(context.TODO()) {
		var result DataTypes.MondadoriBookDocument
		err := cursor.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		filter := bson.M{"ISBN": result.ISBN}
		ebayData := DataTypes.EbayData{
			ISBN:         result.ISBN,
			Published:    result.Published,
			ListingId:    result.ListingId,
			OfferId:      result.OfferId,
			EbayImageURL: result.EbayImageUrl}
		update := bson.M{"$set": ebayData}
		ebayDataModels = append(ebayDataModels, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))

		ebayBook := EbayBookBuilder.BuildEbayBook(result.ISBN)
		update = bson.M{"$set": ebayBook}
		ebayBooksModels = append(ebayBooksModels, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}

	MongoDBInteractions.UpsertEbayData(ebayDataModels)

	MongoDBInteractions.UpsertEbayBooks(ebayBooksModels)
}
