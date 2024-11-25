package main

import (
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

type mongoDBBookDocument struct {
	ISBN      string             `bson:"ISBN"`
	Published bool               `bson:"Published"`
	Title     string             `bson:"Title"`
	Available string             `bson:"Available"`
	Price     string             `bson:"Price"`
	URL       string             `bson:"URL"`
	ImageURL  string             `bson:"ImageURL"`
	Author    string             `bson:"Author"`
	Category  string             `bson:"Category"`
	Editor    string             `bson:"Editor"`
	Variant   string             `bson:"Variant"`
	Language  string             `bson:"Language"`
	UpdatedAt primitive.DateTime `bson:"UpdatedAt"`
}

func disconnectFromMongo() {
	err := client.Disconnect(context.TODO())
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error while disconnecting client.")
		if err != nil {
			fmt.Println(err)
		}
	}
}

var client *mongo.Client
var feltrinelliProductsCollection *mongo.Collection
var feltrinelliBooksCollection *mongo.Collection

func connectToMongo() {
	var clientOptions *options.ClientOptions
	if runtime.GOOS == "windows" {
		clientOptions = options.Client().ApplyURI("mongodb://192.168.188.45:27017/")
	} else {
		clientOptions = options.Client().ApplyURI("mongodb://localhost:27017/")
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
	feltrinelliProductsCollection = client.Database("Mondadori").Collection("FeltrinelliProducts")
	feltrinelliBooksCollection = client.Database("Mondadori").Collection("FeltrinelliBooks")

	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
}

func insertBooks(books []*BookFull) {
	coll := client.Database("Mondadori").Collection("Books")
	if len(books) == 0 {
		return
	}
	nowTime := time.Now()

	var documents []interface{}
	for _, book := range books {
		document := mongoDBBookDocument{
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

	_, err := coll.InsertMany(context.TODO(), documents)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while inserting book documents in MongoDB", err, documents)
		if err != nil {
			panic(err)
		}

	}
}

func bulkInsertBooksToUpdate(books []BookToUpdate) {
	coll := client.Database("Mondadori").Collection("BooksToUpdate")
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

	_, err := coll.BulkWrite(context.TODO(), operations)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while upserting books in MongoDB:", err)
		if err != nil {
			panic(err)
		}
	}
}

func bulkUpdateBooks(books []*BookFull) {
	coll := client.Database("Mondadori").Collection("Books")
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
	_, err := coll.BulkWrite(context.TODO(), models, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk update operation:", err)
		panic(err)
	}
}

func getAllDBBooks(dbBooks map[string]BookFull) {
	startTime := time.Now()
	coll := client.Database("Mondadori").Collection("Books")
	// Find all documents
	cur, err := coll.Find(context.TODO(), bson.D{{"ISBN", bson.D{{"$exists", true}}}})
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
		var result mongoDBBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		dbBooks[result.ISBN] = BookFull{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")
}

func getAllPublishedBooks(dbBooks map[string]BookFull) {
	startTime := time.Now()
	coll := client.Database("Mondadori").Collection("Books")
	// Find all documents
	cur, err := coll.Find(context.TODO(), bson.D{{"ListingId", bson.D{{"$exists", true}}}})
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
		var result mongoDBBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		dbBooks[result.ISBN] = BookFull{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")
}

func setProductIsBook(URL string, isBook bool) {
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
}

func bulkWriteFeltrinelliProducts(models []mongo.WriteModel) {
	_, err := feltrinelliProductsCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}

func formatNumber(number json.Number) (string, error) {
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

func insertFeltrinelliScrapedBook(feltrinelliScrapedBook *FeltrinelliScrapedBook) {
	formattedPrice, err := formatNumber(feltrinelliScrapedBook.BuyInfos.Price)
	if err != nil {
		log.Fatalf("Failed to convert price: %v", err)
	}
	delete(feltrinelliScrapedBook.Details, "EAN")
	document := bson.D{
		{Key: "URL", Value: feltrinelliScrapedBook.BuyInfos.URL},
		{Key: "ISBN", Value: feltrinelliScrapedBook.BuyInfos.ISBN},
		{Key: "Title", Value: feltrinelliScrapedBook.BuyInfos.Title},
		{Key: "AvailabilityStickyText", Value: feltrinelliScrapedBook.BuyInfos.AvailabilityStickyText},
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

	setProductIsBook(feltrinelliScrapedBook.BuyInfos.URL, true)
}
