package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type mongoDBBookDocument struct {
	ISBN      string `bson:"ISBN"`
	URL       string `bson:"URL"`
	Title     string `bson:"Title"`
	ImageURL  string `bson:"ImageURL"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
	Author    string `bson:"Author"`
	Category  string `bson:"Category"`
	Variant   string `bson:"Variant"`
	Editor    string `bson:"Editor"`
	Language  string `bson:"Language"`
}

type mongoDBBookToUpdateDocument struct {
	ISBN      string `bson:"ISBN"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
}

func connectToMongo(uri string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	fmt.Println("Connected to MongoDB!")
	return client, nil
}

func insertBook(coll *mongo.Collection, book BookFull) {
	document := mongoDBBookDocument{
		ISBN:      book.ISBN,
		URL:       book.URL,
		ImageURL:  book.ImageURL,
		Price:     book.Price,
		Available: book.Available,
		Title:     book.Title,
		Author:    book.Author,
		Category:  book.Category,
		Language:  book.Language,
		Variant:   book.Variant,
		Editor:    book.Editor,
	}
	_, err := coll.InsertOne(context.TODO(), document)
	if err != nil {
		fmt.Println("Error occurred while inserting book document in MongoDB", err)
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
		fmt.Println("Error occurred while inserting partial book document in MongoDB", err)
	}
}

func getAllDB(coll *mongo.Collection, dbBooks map[string]BookPartial) {
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
		dbBooks[result.ISBN] = BookPartial{ISBN: result.ISBN, Available: result.Available, Price: result.Price}
	}
}
