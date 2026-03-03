package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetAllFeltrinelliBooksOnEbay() map[string]DataTypes.EbayDataWithFeltrinelliBook {
	startTime := time.Now()

	// Define the aggregation pipeline
	pipeline := bson.A{
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "EbayData", Value: "$$ROOT"},
			}},
		},
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "FeltrinelliBooks"},
				{Key: "localField", Value: "EbayData.ISBN"},
				{Key: "foreignField", Value: "ISBN"},
				{Key: "as", Value: "FeltrinelliBook"},
			}},
		},
		bson.D{
			{Key: "$unwind", Value: bson.D{
				{Key: "path", Value: "$FeltrinelliBook"},
			}},
		},
	}

	// Execute the aggregation pipeline
	cursor, err := ebayDataCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {

		}
	}(cursor, context.TODO())

	// Iterate through the results
	var results []DataTypes.EbayDataWithFeltrinelliBook
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Finished getting", len(results), "Feltrinelli books on Ebay in", time.Since(startTime).Seconds(), "seconds")

	outputMap := make(map[string]DataTypes.EbayDataWithFeltrinelliBook, len(results))
	for _, result := range results {
		outputMap[result.FeltrinelliBook.ISBN] = result
	}
	return outputMap
}

func SetProductIsBook(URL string, isBook bool) {
	filter := bson.M{"URL": URL}

	update := bson.M{
		"$set": bson.M{
			"IsBook": isBook,
		},
	}
	_, err := feltrinelliProductsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Fatalf("Failed to set product is book: %v", err)
	}
	deleteFeltrinelliBook(URL)
}

func deleteFeltrinelliBook(URL string) {
	filter := bson.M{"URL": URL}
	_, err := feltrinelliBooksCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		log.Fatalf("Failed to delete feltrinelli book: %v", err)
	}
}

// FormatNumberIntoString Formats a json.Number, so an integer (ex. 1234, 345), into a formatted string,
// representing a float(ex. 12,34, 3,45).
func FormatNumberIntoString(number json.Number) (string, error) {
	// Parse the JSON number into an integer
	num, err := number.Int64()
	if err != nil {
		return "", fmt.Errorf("failed to parse number: %v", err)
	}

	// Convert the integer to a string
	numStr := strconv.FormatInt(num, 10)

	// Ensure the number has at least 2 digits for formatting
	if len(numStr) < 2 {
		numStr = "0" + numStr
	}

	// Split the string to insert a comma
	n := len(numStr)
	formatted := numStr[:n-2] + "," + numStr[n-2:]

	return formatted, nil
}

func BulkWriteFeltrinelliProducts(models []mongo.WriteModel) {
	_, err := feltrinelliProductsCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}

func InsertFeltrinelliScrapedBook(feltrinelliScrapedBook *DataTypes.FeltrinelliScrapedBook) {
	formattedPrice, err := FormatNumberIntoString(feltrinelliScrapedBook.BuyInfos.Price)
	if err != nil {
		log.Fatalf("Failed to convert price: %v", err)
	}
	delete(feltrinelliScrapedBook.Details, "EAN")
	document := bson.D{
		{Key: "URL", Value: feltrinelliScrapedBook.BuyInfos.URL},
		{Key: "ISBN", Value: feltrinelliScrapedBook.BuyInfos.ISBN},
		{Key: "Title", Value: feltrinelliScrapedBook.BuyInfos.Title},
		{Key: "Availability", Value: feltrinelliScrapedBook.BuyInfos.Availability},
		{Key: "Price", Value: formattedPrice},
		{Key: "Details", Value: feltrinelliScrapedBook.Details},
		{Key: "LongDescription", Value: feltrinelliScrapedBook.DescriptionData.LongDescription},
		{Key: "ShortDescription", Value: feltrinelliScrapedBook.DescriptionData.ShortDescription},
		{Key: "Category", Value: feltrinelliScrapedBook.Category},
	}

	filter := bson.M{"ISBN": feltrinelliScrapedBook.BuyInfos.ISBN}
	update := bson.M{"$set": document}
	// Enable upsert
	opts := options.Update().SetUpsert(true)

	// Perform the update operation
	_, err = feltrinelliBooksCollection.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		log.Fatalf("Failed to insert feltrinelli scraped book: %v", err)
	}

	SetProductIsBook(feltrinelliScrapedBook.BuyInfos.URL, true)
}

func RemoveAllUnseenProductsAndBooksFeltrinelli(lastSeen time.Time) {
	fmt.Println("Removing all unseen Feltrinelli products.")
	startTime := time.Now()
	filter := bson.M{
		"LastSeen": bson.M{
			"$ne": lastSeen,
		},
	}

	cursor, err := feltrinelliProductsCollection.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while closing cursor", err)
			panic(err)
		}
	}(cursor, context.TODO())

	deleteModels := make([]mongo.WriteModel, 0)
	for cursor.Next(context.TODO()) {
		var result DataTypes.FeltrinelliProduct
		err := cursor.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}
		model := mongo.NewDeleteOneModel()
		model.SetFilter(bson.M{"URL": result.URL})
		deleteModels = append(deleteModels, model)
	}

	if len(deleteModels) == 0 {
		return
	}

	fmt.Printf("Removing %d products.\n", len(deleteModels))

	bulkOption := options.BulkWrite().SetOrdered(false)

	// DELETING BOOKS MUST ALWAYS HAPPEN BEFORE DELETING PRODUCTS!!!
	// Delete books with given URLs
	_, err = feltrinelliBooksCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Delete products with given URLs
	_, err = feltrinelliProductsCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	fmt.Println("Removed all unseen Feltrinelli products and books in ", time.Since(startTime).Seconds(), "seconds.")
}

func BuildFeltrinelliPriceOrAvailabilityUpdateModel(bookPartial DataTypes.BookPartial) mongo.WriteModel {
	model := mongo.NewUpdateOneModel()
	model.SetFilter(bson.M{"ISBN": bookPartial.ISBN})
	model.SetUpdate(bson.M{"$set": bson.M{"Price": bookPartial.Price, "Availability": bookPartial.Available}})
	return model
}

func UpdateFeltrinelliPriceOrAvailability(models []mongo.WriteModel) {
	if len(models) == 0 {
		return
	}

	res, err := feltrinelliBooksCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Inserted %v and modified %v feltrinelli books\n", res.InsertedCount, res.ModifiedCount)
}

func GetFeltrinelliBook(isbn string) (*DataTypes.FeltrinelliBook, error) {
	filter := bson.M{"ISBN": isbn}
	var result DataTypes.FeltrinelliBook
	err := feltrinelliBooksCollection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func DeleteFeltrinelliBook(URL string) {
	filter := bson.M{"URL": URL}
	result, err := feltrinelliBooksCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		panic(err)
	}
	if result != nil && result.DeletedCount < 1 {
		panic("Feltrinelli book was not deleted. URL:" + URL)
	}
}

func DeleteFeltrinelliProduct(URL string) {
	filter := bson.M{"URL": URL}
	result, err := feltrinelliProductsCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		panic(err)
	}
	if result != nil && result.DeletedCount < 1 {
		panic("Feltrinelli product was not deleted. URL:" + URL)
	}
}

func GetFeltrinelliProduct(URL string) (*DataTypes.FeltrinelliProduct, error) {
	filter := bson.M{"URL": URL}
	var result DataTypes.FeltrinelliProduct
	err := feltrinelliProductsCollection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func SetBookIsRedirected(URL string) {
	filter := bson.M{"URL": URL}
	update := bson.M{"$set": bson.M{"Availability": "Redirected"}}
	_, err := feltrinelliBooksCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		panic(err)
	}
}

func getEAN(s string) string {
	lastSlashIndex := strings.LastIndex(s, "/")

	if lastSlashIndex != -1 {
		return s[lastSlashIndex+1:]
	}

	panic("No '/' found in the string.")
}

func BuildFeltrinelliProductUpsertModel(entry DataTypes.URL, lastSeen time.Time) *mongo.UpdateOneModel {
	filter := bson.M{"URL": entry.Loc}

	update := bson.M{
		"$set": bson.M{
			"URL":      entry.Loc,
			"EAN":      getEAN(entry.Loc),
			"LastSeen": lastSeen,
		},
	}

	model := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(update).
		SetUpsert(true)
	return model
}

func GetAllBookProductsAndNewProductsURLsIntoChannel(urlsChan chan<- string) {
	filter := bson.M{
		"$or": []bson.M{
			{"IsBook": true},
			{"IsBook": bson.M{"$exists": false}},
		},
	}

	documentsCount, _ := feltrinelliProductsCollection.CountDocuments(context.TODO(), filter)
	fmt.Println("Number of new products: ", documentsCount)

	SendURLsToChannelFromCursor(urlsChan, filter)
}

func SendURLsToChannelFromCursor(urlsChan chan<- string, filter bson.M) {
	opts := options.Find().SetBatchSize(1000)
	cursor, err := feltrinelliProductsCollection.Find(context.TODO(), filter, opts)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cursor *mongo.Cursor) {
		err := cursor.Close(context.TODO())
		if err != nil {
			panic(err)
		}
	}(cursor)

	startTime := time.Now()
	count := 0
	// Iterate through the cursor and send documents to the channel
	for cursor.Next(context.TODO()) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			// Log the error but continue processing other documents
			log.Printf("Error decoding document: %v\n", err)
			continue
		}

		// Safely extract URL and EAN from the document
		URL, okURL := document["URL"].(string)
		ean := URL[len(URL)-13:]

		if !okURL {
			log.Fatalf("Missing or invalid URL in document: %v\n", document)
		}
		if !strings.HasPrefix(ean, "978") && !strings.HasPrefix(ean, "979") {
			SetProductIsBook(URL, false)
			continue
		}
		urlsChan <- URL
		count++
		if count%100 == 0 {
			newNow := time.Now()
			fmt.Println(count, "Feltrinelli products processed. 100 done in", newNow.Sub(startTime).Seconds())
			startTime = newNow
		}
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	close(urlsChan) // Close the channel when done
}
