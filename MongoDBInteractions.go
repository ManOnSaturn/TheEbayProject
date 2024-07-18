package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"time"
)

type mongoDBBookDocument struct {
	ISBN      string             `bson:"ISBN"`
	Published bool               `bson:"Published"`
	Title     string             `bson:"Title"`
	Available bool               `bson:"Available"`
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

type mongoDBBookToUpdateDocument struct {
	ISBN      string `bson:"ISBN"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
}

func connectToMongo() *mongo.Client {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		fmt.Println("Error occurred while trying to connect to MongoDB")
		panic(err)
	}

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
	return client
}

func insertBooks(coll *mongo.Collection, books []*BookFull) {
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

func bulkInsertBooksToUpdate(coll *mongo.Collection, books []*BookPartial) {
	if len(books) == 0 {
		return
	}

	var documents []interface{}
	for _, book := range books {
		document := mongoDBBookToUpdateDocument{
			ISBN:      book.ISBN,
			Price:     book.Price,
			Available: book.Available,
		}
		documents = append(documents, document)
	}

	_, err := coll.InsertMany(context.TODO(), documents)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while inserting books to update in MongoDB", err, documents)
		if err != nil {
			panic(err)
		}
	}
}

func bulkUpdateBooks(coll *mongo.Collection, books []*BookPartial) {
	if len(books) == 0 {
		return
	}

	var models []mongo.WriteModel
	nowTime := time.Now()
	for _, book := range books {
		filter := bson.M{"ISBN": book.ISBN}
		update := bson.M{
			"$set": bson.M{
				"Price":     book.Price,
				"Available": book.Available,
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

func getAllDBBooks(coll *mongo.Collection, dbBooks map[string]DBBook) {
	startTime := time.Now()
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

		dbBooks[result.ISBN] = DBBook{ISBN: result.ISBN, Available: result.Available, Price: result.Price, Found: false}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")

}
