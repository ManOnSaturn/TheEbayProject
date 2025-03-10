package main

import (
	"Scraper/AmazonScraping"
	"Scraper/EbayBookBuilder"
	"Scraper/FeltrinelliScraping"
	"Scraper/MondadoriScraping"
	"Scraper/MongoDBInteractions"
	"Scraper/PythonInteractions"
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

	if len(os.Args) > 1 && os.Args[1] == "--repricer" {
		reprice()
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

	//addstuf()
	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}

func addstuf() {
	ebayDataWithMondadoriBooks := MongoDBInteractions.GetAllMondadoriBooksOnEbay()
	var models []mongo.WriteModel

	for isbn, ebayDataWithMondadoriBook := range ebayDataWithMondadoriBooks {
		filter := bson.M{"ISBN": isbn}
		categoryID := EbayBookBuilder.GetCategoryIDMondadori(ebayDataWithMondadoriBook.MondadoriBook.Categories[0])
		update := bson.M{"$set": bson.M{"CategoryID": categoryID, "ImageURL": ebayDataWithMondadoriBook.MondadoriBook.ImageURL}}
		updateModel := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
		models = append(models, updateModel)
	}

	MongoDBInteractions.UpsertEbayBooks(models)
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

	ebayDatas := MongoDBInteractions.GetAllMondadoriBooksOnEbay()
	isbns := make([]string, 0)
	for _, ebayData := range ebayDatas {
		isbns = append(isbns, ebayData.EbayData.ISBN)
	}

	newEbayBooks := EbayBookBuilder.BuildEbayBooks(isbns)
	oldEbayBooks := MongoDBInteractions.GetAllEbayBooks()

	var updateModels []mongo.WriteModel
	for _, oldEbayBook := range oldEbayBooks {
		if newEbayBook, ok := newEbayBooks[oldEbayBook.ISBN]; !ok {
			// For some reason, we didn't get the book created. We don't know what happened, therefore we delete the
			// entry from ebay completely.
			MongoDBInteractions.AddBookToUpdate(oldEbayBook.ISBN)
			MongoDBInteractions.DeleteEbayBook(oldEbayBook.ISBN)
		} else if !newEbayBook.Equals(oldEbayBook) {
			MongoDBInteractions.AddBookToUpdate(oldEbayBook.ISBN)
			updateModels = append(updateModels, MongoDBInteractions.CreateUpsertModelForEbayBooks(newEbayBook))
		}
	}

	MongoDBInteractions.UpsertEbayBooks(updateModels)
	PythonInteractions.StartPythonRepricer(len(updateModels), false)
}
