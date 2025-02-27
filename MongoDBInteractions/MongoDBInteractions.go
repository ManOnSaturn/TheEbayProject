package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

func DisconnectFromMongo() {
	err := client.Disconnect(context.TODO())
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error while disconnecting client.")
		if err != nil {
			fmt.Println(err)
		}
	}
}

var client *mongo.Client
var FeltrinelliProductsCollection *mongo.Collection
var feltrinelliBooksCollection *mongo.Collection
var booksCollection *mongo.Collection
var booksToUpdateCollection *mongo.Collection
var feltrinelliBooksToUpdateCollection *mongo.Collection
var BestsellersAmazonCollection *mongo.Collection
var ebayDataCollection *mongo.Collection

func ConnectToMongo() {
	var clientOptions *options.ClientOptions
	if runtime.GOOS == "windows" {
		clientOptions = options.Client().ApplyURI("mongodb://admin:asdfadfhxvbxbsdfghs@192.168.188.45:30000/admin")
	} else {
		clientOptions = options.Client().ApplyURI("mongodb://admin:asdfadfhxvbxbsdfghs@localhost:30000/admin")
	}
	var err error
	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		fmt.Println("Error occurred while trying to connect to MongoDB")
		panic(err)
	}

	// Send a ping to confirm a successful connection
	if err = client.Database("admin").RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	FeltrinelliProductsCollection = client.Database("Mondadori").Collection("FeltrinelliProducts")
	feltrinelliBooksCollection = client.Database("Mondadori").Collection("FeltrinelliBooks")
	booksCollection = client.Database("Mondadori").Collection("Books")
	booksToUpdateCollection = client.Database("Mondadori").Collection("BooksToUpdate")
	feltrinelliBooksToUpdateCollection = client.Database("Mondadori").Collection("FeltrinelliBooksToUpdate")
	BestsellersAmazonCollection = client.Database("Mondadori").Collection("BestsellersAmazon")
	ebayDataCollection = client.Database("Mondadori").Collection("EbayData")

	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
}

func InsertBooks(books []*DataTypes.BookFull) {
	if len(books) == 0 {
		return
	}
	nowTime := time.Now()

	var documents []interface{}
	for _, book := range books {
		document := DataTypes.MongoDBBookDocument{
			ISBN:      book.ISBN,
			Published: false,
			Title:     book.Title,
			Available: book.Available,
			Price:     book.Price,
			URL:       book.URL,
			ImageURL:  book.ImageURL,
			Author:    book.Author,
			Category:  book.Category,
			Editor:    book.Editor,
			Variant:   book.Variant,
			Language:  book.Language,
			UpdatedAt: primitive.NewDateTimeFromTime(nowTime),
		}
		documents = append(documents, document)
	}

	_, err := booksCollection.InsertMany(context.TODO(), documents)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while inserting book documents in MongoDB", err, documents)
		if err != nil {
			panic(err)
		}

	}
}

func BulkInsertBooksToUpdate(books []DataTypes.BookToUpdate, isFeltrinelli bool) {
	if len(books) == 0 {
		return
	}

	var operations []mongo.WriteModel
	for _, book := range books {
		filter := bson.M{"ISBN": book.ISBN}
		update := bson.M{"$set": bson.M{
			"Price":               book.Price,
			"PriceChanged":        book.PriceChanged,
			"Available":           book.Available,
			"AvailabilityChanged": book.AvailabilityChanged,
		}}
		upsert := mongo.NewUpdateOneModel()
		upsert.SetFilter(filter)
		upsert.SetUpdate(update)
		upsert.SetUpsert(true)
		operations = append(operations, upsert)
	}
	var collection *mongo.Collection
	if isFeltrinelli {
		collection = feltrinelliBooksToUpdateCollection
	} else {
		collection = booksToUpdateCollection
	}
	_, err := collection.BulkWrite(context.TODO(), operations)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while upserting books in MongoDB:", err)
		if err != nil {
			panic(err)
		}
	}
}

func BulkUpdateBooks(books []*DataTypes.BookFull) {
	if len(books) == 0 {
		return
	}

	var models []mongo.WriteModel
	nowTime := time.Now()
	for _, book := range books {
		filter := bson.M{"ISBN": book.ISBN}
		update := bson.M{
			"$set": bson.M{
				"ISBN":      book.ISBN,
				"Title":     book.Title,
				"Available": book.Available,
				"Price":     book.Price,
				"URL":       book.URL,
				"ImageURL":  book.ImageURL,
				"Author":    book.Author,
				"Category":  book.Category,
				"Editor":    book.Editor,
				"Variant":   book.Variant,
				"Language":  book.Language,
				"UpdatedAt": primitive.NewDateTimeFromTime(nowTime),
			},
		}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
		models = append(models, model)
	}

	bulkOption := options.BulkWrite().SetOrdered(false)
	_, err := booksCollection.BulkWrite(context.TODO(), models, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk update operation:", err)
		panic(err)
	}
}

func GetAllDBBooks(dbBooks map[string]DataTypes.BookFull) {
	startTime := time.Now()
	// Find all documents
	cur, err := booksCollection.Find(context.TODO(), bson.D{{"ISBN", bson.D{{"$exists", true}}}})
	if err != nil {
		log.Fatal(err)
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while closing cursor", err)
			panic(err)
		}
	}(cur, context.TODO())

	for cur.Next(context.TODO()) {
		var result DataTypes.MongoDBBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		dbBooks[result.ISBN] = DataTypes.BookFull{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")
}

func GetAllPublishedBooks() map[string]DataTypes.BookFull {
	startTime := time.Now()

	booksFromDB := make(map[string]DataTypes.BookFull, 5000)
	// Find all documents
	cur, err := booksCollection.Find(context.TODO(), bson.D{{"ListingId", bson.D{{"$exists", true}}}})
	if err != nil {
		log.Fatal(err)
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while closing cursor", err)
			panic(err)
		}
	}(cur, context.TODO())

	for cur.Next(context.TODO()) {
		var result DataTypes.MongoDBBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		booksFromDB[result.ISBN] = DataTypes.BookFull{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}

	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")

	return booksFromDB
}

func GetAllBooksOnEbay() map[string]DataTypes.EbayDataWithFeltrinelliBook {
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
				{Key: "path", Value: "FeltrinelliBook"},
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

	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")

	outputMap := make(map[string]DataTypes.EbayDataWithFeltrinelliBook, len(results))
	for _, result := range results {
		outputMap[result.FeltrinelliBook.ISBN] = DataTypes.EbayDataWithFeltrinelliBook{FeltrinelliBook: result.FeltrinelliBook, EbayData: result.EbayData}
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
	_, err := FeltrinelliProductsCollection.UpdateOne(context.TODO(), filter, update)
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

func BulkWriteFeltrinelliProducts(models []mongo.WriteModel) {
	_, err := FeltrinelliProductsCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}

func FormatNumber(number json.Number) (string, error) {
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

func InsertFeltrinelliScrapedBook(feltrinelliScrapedBook *DataTypes.FeltrinelliScrapedBook) {
	formattedPrice, err := FormatNumber(feltrinelliScrapedBook.BuyInfos.Price)
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

func SetProductProblematic(URL string, isProblematic bool) {
	filter := bson.M{"URL": URL}

	update := bson.M{
		"$set": bson.M{
			"IsProblematic": isProblematic,
		},
	}
	_, err := FeltrinelliProductsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Fatalf("Failed to set product is book: %v", err)
	}
}

func SetNewURLAndIsBook(originalURL string, URL string, isBook bool) {
	filter := bson.M{"URL": originalURL}

	update := bson.M{
		"$set": bson.M{
			"URL":    URL,
			"IsBook": isBook,
		},
		"$push": bson.M{
			"PreviousURLs": originalURL,
		},
	}
	_, err := FeltrinelliProductsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Fatalf("Failed to update product: %v", err)
	}

	updateBook := bson.M{
		"$set": bson.M{
			"URL": URL,
		},
	}
	_, err = feltrinelliBooksCollection.UpdateOne(context.TODO(), filter, updateBook)
	if err != nil {
		log.Fatalf("Failed to update URL for book: %v", err)
	}
}

func RemoveAllUnseenProductsAndBooks(lastSeen time.Time) {
	fmt.Println("Removing all unseen products")
	startTime := time.Now()
	filter := bson.M{
		"LastSeen": bson.M{
			"$ne": lastSeen,
		},
	}

	cursor, err := FeltrinelliProductsCollection.Find(context.TODO(), filter)
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
	feltrinelliBooksISBNs := make([]string, 0)
	for cursor.Next(context.TODO()) {
		var result DataTypes.MongoDBFeltrinelliProduct
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
		var feltrinelliBook DataTypes.FeltrinelliBook
		findOneError := feltrinelliBooksCollection.FindOne(context.TODO(), bson.M{"URL": result.URL}).Decode(&feltrinelliBook)
		if findOneError == nil {
			feltrinelliBooksISBNs = append(feltrinelliBooksISBNs, feltrinelliBook.ISBN)
		}
	}

	if len(deleteModels) == 0 {
		return
	}

	fmt.Printf("Removing %d products.\n", len(deleteModels))

	bulkOption := options.BulkWrite().SetOrdered(false)

	// Delete products with given URLs
	_, err = FeltrinelliProductsCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Delete books with given URLs
	_, err = feltrinelliBooksCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Change availability if book is published
	var booksToUpdateModels []mongo.WriteModel
	for _, ISBN := range feltrinelliBooksISBNs {
		documents, _ := ebayDataCollection.CountDocuments(context.TODO(), bson.M{"ISBN": ISBN})
		if documents > 0 {
			model := mongo.NewUpdateOneModel()
			model.SetFilter(bson.M{"ISBN": ISBN})
			model.SetUpsert(true)
			model.SetUpdate(bson.M{"$set": bson.M{"ISBN": ISBN, "Available": "Rimosso", "AvailabilityChanged": true, "Price": "", "PriceChanged": false}})
			booksToUpdateModels = append(booksToUpdateModels, model)
		}
	}

	// TODO reason about whether this is actually needed, or needs to be checked during repricing, or both
	if len(booksToUpdateModels) > 0 {
		fmt.Println("Storing books which disappeared as books to update")
		_, err = feltrinelliBooksToUpdateCollection.BulkWrite(context.TODO(), booksToUpdateModels)
		if err != nil {
			_, err = fmt.Fprintln(os.Stderr, "Error occurred during bulk write operation:", err)
			return
		}
	}

	fmt.Println("Removed all unseen products in ", time.Since(startTime).Seconds(), "seconds.")
}
