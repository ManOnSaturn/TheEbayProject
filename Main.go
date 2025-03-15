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
	wg.Add(2)
	go func() {
		defer wg.Done()
		FeltrinelliScraping.Repricer()
	}()
	go func() {
		defer wg.Done()
		MondadoriScraping.Repricer()
	}()
	wg.Wait()

	isbns := getISBNsOfBooksOnEbay()

	newEbayBooks := EbayBookBuilder.BuildEbayBooks(isbns)
	oldEbayBooks := MongoDBInteractions.GetAllEbayBooks()

	var updateModels []mongo.WriteModel
	for _, oldEbayBook := range oldEbayBooks {
		if newEbayBook, ok := newEbayBooks[oldEbayBook.ISBN]; !ok {
			// For some reason, we didn't get the book created. We don't know what happened.
			MongoDBInteractions.AddBookToUpdate(oldEbayBook.ISBN)
			updateModels = append(updateModels, MongoDBInteractions.CreateUpsertModelForEbayBooks(oldEbayBook))
		} else if !newEbayBook.Equals(oldEbayBook) {
			MongoDBInteractions.AddBookToUpdate(oldEbayBook.ISBN)
			updateModels = append(updateModels, MongoDBInteractions.CreateUpsertModelForEbayBooks(newEbayBook))
		}
	}

	MongoDBInteractions.UpsertEbayBooks(updateModels)
	PythonInteractions.StartPythonRepricer(len(updateModels))
}

func getISBNsOfBooksOnEbay() map[string]bool {
	isbns := make(map[string]bool)
	mondadoriBooksOnEbay := MongoDBInteractions.GetAllMondadoriBooksOnEbay()
	for _, ebayData := range mondadoriBooksOnEbay {
		isbns[ebayData.EbayData.ISBN] = true
	}
	feltrinelliBooksOnEbay := MongoDBInteractions.GetAllFeltrinelliBooksOnEbay()
	for _, ebayData := range feltrinelliBooksOnEbay {
		isbns[ebayData.EbayData.ISBN] = true
	}
	return isbns
}
