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

func connectToMongo(uri string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	return client, nil
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

	_, err := coll.InsertMany(context.TODO(), documents)
	if err != nil {
		fmt.Println("Error occurred while inserting book documents in MongoDB", err, documents)
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
		fmt.Println("Error occurred while inserting partial book document in MongoDB", err, document)
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

func getAllDB(coll *mongo.Collection, dbBooks map[string]DBBook) {
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
}
