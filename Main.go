package main

import (
	"Scraper/AmazonScraping"
	"Scraper/DataTypes"
	"Scraper/Ebay"
	"Scraper/EbayBookBuilder"
	"Scraper/FeltrinelliScraping"
	"Scraper/MondadoriScraping"
	"Scraper/MongoDBInteractions"
	"Scraper/PythonInteractions"
	"fmt"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeMondadori" {
		MondadoriScraping.FullScrape()
	}
	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeFeltrinelli" {
		FeltrinelliScraping.FullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--deleteDisappearedBooksFromFile" {
		deleteDisappearedBooksFromFile("")
	}

	if len(os.Args) > 1 && os.Args[1] == "--deleteAllBooksFromEbay" {
		items, err := Ebay.GetInventoryItems()
		if err != nil {
			return
		}
		wg := sync.WaitGroup{}
		semaphore := DataTypes.NewSemaphore(10)
		for _, item := range items {
			semaphore.Acquire()
			wg.Add(1)
			go func() {
				defer func() {
					wg.Done()
					semaphore.Release()
				}()
				deleteBookFromEbay(item.SKU)
			}()
		}
		wg.Wait()
	}

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

func deleteDisappearedBooksFromFile(isbn string) {
	// The sequence of operations doesn't seem safe.
	deleteBookFromEbay(isbn)
	deleteMondadoriBookAndProduct(isbn)
	deleteFeltrinelliBookAndProduct(isbn)
}

func deleteBookFromEbay(isbn string) {
	ebayData := MongoDBInteractions.GetEbayData(isbn)
	if ebayData == nil {
		return
	}
	Ebay.DeleteOffer(ebayData.OfferId, false)
	Ebay.DeleteInventoryItem(isbn, false)
	MongoDBInteractions.DeleteEbayData(isbn)
	MongoDBInteractions.DeleteEbayBook(isbn)
}

func deleteMondadoriBookAndProduct(isbn string) {
	mondadoriBook, _ := MongoDBInteractions.GetMondadoriBook(isbn)
	var mondadoriProductURL *string
	if mondadoriBook != nil {
		mondadoriProductURL = &mondadoriBook.URL
	} else {
		mondadoriProduct, _ := MongoDBInteractions.GetMondadoriProduct(isbn)
		if mondadoriProduct != nil {
			mondadoriProductURL = &mondadoriProduct.URL
		}
	}
	if mondadoriProductURL != nil {
		MongoDBInteractions.DeleteMondadoriBook(*mondadoriProductURL)
		MongoDBInteractions.DeleteMondadoriProduct(*mondadoriProductURL)
	}
}

func deleteFeltrinelliBookAndProduct(isbn string) {
	feltrinelliBook, _ := MongoDBInteractions.GetFeltrinelliBook(isbn)
	var feltrinelliProductURL *string
	if feltrinelliBook != nil {
		feltrinelliProductURL = &feltrinelliBook.URL
	} else {
		feltrinelliProduct, _ := MongoDBInteractions.GetFeltrinelliProduct(isbn)
		if feltrinelliProduct == nil {
			return
		}
		feltrinelliProductURL = &feltrinelliProduct.URL
	}
	if feltrinelliProductURL != nil {
		MongoDBInteractions.DeleteFeltrinelliBook(*feltrinelliProductURL)
		MongoDBInteractions.DeleteFeltrinelliProduct(*feltrinelliProductURL)
	}
}
