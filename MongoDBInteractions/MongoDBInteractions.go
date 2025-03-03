package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"runtime"
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
var mondadoriProductsCollection *mongo.Collection
var feltrinelliBooksCollection *mongo.Collection
var mondadoriBooksCollection *mongo.Collection
var feltrinelliBooksToUpdateCollection *mongo.Collection
var mondadoriBooksToUpdateCollection *mongo.Collection
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

	database := client.Database("Rimanga")
	//database := client.Database("Mondadori")
	// Send a ping to confirm a successful connection

	if err = database.RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}

	FeltrinelliProductsCollection = database.Collection("FeltrinelliProducts")
	mondadoriProductsCollection = database.Collection("MondadoriProducts")
	feltrinelliBooksCollection = database.Collection("FeltrinelliBooks")
	mondadoriBooksCollection = database.Collection("Books")
	mondadoriBooksToUpdateCollection = database.Collection("BooksToUpdate")
	feltrinelliBooksToUpdateCollection = database.Collection("FeltrinelliBooksToUpdate")
	BestsellersAmazonCollection = database.Collection("BestsellersAmazon")
	ebayDataCollection = database.Collection("EbayData")

	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
}

func InsertBooks(books []*DataTypes.MondadoriBook) {
	if len(books) == 0 {
		return
	}
	nowTime := time.Now()

	var documents []interface{}
	for _, book := range books {
		document := DataTypes.MondadoriBookDocument{
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

	_, err := mondadoriBooksCollection.InsertMany(context.TODO(), documents)
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
		collection = mondadoriBooksToUpdateCollection
	}
	_, err := collection.BulkWrite(context.TODO(), operations)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while upserting books in MongoDB:", err)
		if err != nil {
			panic(err)
		}
	}
}

func BulkUpdateBooks(books []*DataTypes.MondadoriBook) {
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
	_, err := mondadoriBooksCollection.BulkWrite(context.TODO(), models, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk update operation:", err)
		panic(err)
	}
}

func GetAllDBBooks(dbBooks map[string]DataTypes.MondadoriBook) {
	startTime := time.Now()
	// Find all documents
	cur, err := mondadoriBooksCollection.Find(context.TODO(), bson.D{{"ISBN", bson.D{{"$exists", true}}}})
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
		var result DataTypes.MondadoriBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		dbBooks[result.ISBN] = DataTypes.MondadoriBook{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")
}

func GetAllPublishedBooks() map[string]DataTypes.MondadoriBook {
	startTime := time.Now()

	booksFromDB := make(map[string]DataTypes.MondadoriBook, 5000)
	// Find all documents
	cur, err := mondadoriBooksCollection.Find(context.TODO(), bson.D{{"ListingId", bson.D{{"$exists", true}}}})
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
		var result DataTypes.MondadoriBookDocument
		err := cur.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		booksFromDB[result.ISBN] = DataTypes.MondadoriBook{ISBN: result.ISBN, Published: result.Published, Title: result.Title,
			Available: result.Available, Price: result.Price, URL: result.URL, ImageURL: result.ImageURL,
			Author: result.Author, Category: result.Category, Variant: result.Variant, Editor: result.Editor,
			Language: result.Language}
	}

	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")

	return booksFromDB
}
