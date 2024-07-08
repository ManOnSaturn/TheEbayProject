package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"time"
)

type mongoDBBookDocument struct {
	ISBN      string `bson:"ISBN"`
	Published bool   `bson:"Published"`
	Title     string `bson:"Title"`
	Available bool   `bson:"Available"`
	Price     string `bson:"Price"`
	URL       string `bson:"URL"`
	ImageURL  string `bson:"ImageURL"`
	Author    string `bson:"Author"`
	Category  string `bson:"Category"`
	Editor    string `bson:"Editor"`
	Variant   string `bson:"Variant"`
	Language  string `bson:"Language"`
}

type mongoDBBookToUpdateDocument struct {
	ISBN      string `bson:"ISBN"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
}

func connectToMongo() *mongo.Client {
	// "mongodb://localhost:27017/"
	clientOptions := options.Client().ApplyURI("mongodb+srv://admin:wTajho41SAXpouc2@cluster0.pmtj7nf.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0")
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

func insertBooks(coll *mongo.Collection, books []BookFull) {
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
		}
		documents = append(documents, document)
	}

	if len(documents) > 0 {
		_, err := coll.InsertMany(context.TODO(), documents)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while inserting book documents in MongoDB", err, documents)
			if err != nil {
				return
			}
		}
	}
}

func insertBookToUpdate(coll *mongo.Collection, book BookPartial) {
	document := mongoDBBookToUpdateDocument{
		ISBN:      book.ISBN,
		Price:     book.Price,
		Available: book.Available,
	}
	_, err := coll.InsertOne(context.TODO(), document)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while inserting book to update document in MongoDB", err, document)
		if err != nil {
			return
		}
	}
}

func updateBookToUpdate(coll *mongo.Collection, book BookPartial) {
	filter := bson.M{"isbn": book.ISBN}
	update := bson.M{
		"$set": bson.M{
			"Price":     book.Price,
			"Available": book.Available,
		},
	}

	result := coll.FindOneAndUpdate(context.TODO(), filter, update)
	if result.Err() != nil {
		fmt.Println("Error occurred while updating partial book document in MongoDB", result.Err(), filter, update)
	}
}

func getAllDBBooks(coll *mongo.Collection, dbBooks map[string]DBBook) {
	startTime := time.Now()
	// Find all documents
	cur, err := coll.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			fmt.Println("Error occurred while closing cursor", err)
		}
	}(cur, context.TODO())

	for cur.Next(context.TODO()) {
		var result mongoDBBookDocument
		err := cur.Decode(&result)
		if err != nil {
			fmt.Println("Error occurred while decoding result", err)
		}
		dbBooks[result.ISBN] = DBBook{ISBN: result.ISBN, Available: result.Available, Price: result.Price, Found: false}
	}
	fmt.Println("Finished getting all books in", time.Since(startTime).Seconds(), "seconds")

}
