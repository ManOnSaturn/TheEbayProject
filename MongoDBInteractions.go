package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoDBBookDocument struct {
	URL       string `bson:"URL"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
	ISBN      string `bson:"ISBN"`
	Title     string `bson:"Title"`
	Author    string `bson:"Author"`
	Category  string `bson:"Category"`
	Language  string `bson:"Language"`
	Editor    string `bson:"Editor"`
}

type mongoDBImageDocument struct {
	BookURL string `bson:"BookURL"`
	Image   []byte `bson:"Image"`
}

func connectToMongo(uri string) (*mongo.Client, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	fmt.Println("Connected to MongoDB!")
	return client, nil
}

func insertBook(coll *mongo.Collection, book FullBookInfo) {
	document := mongoDBBookDocument{
		URL:       book.URL,
		Price:     book.Price,
		Available: book.Available,
		ISBN:      book.ISBN,
		Title:     book.Title,
		Author:    book.Author,
		Category:  book.Category,
		Language:  book.Language,
		Editor:    book.Editor,
	}
	_, err := coll.InsertOne(context.TODO(), document)
	if err != nil {
		fmt.Println("Error occurred while inserting book document in MongoDB", err)
	}
}

func insertImage(coll *mongo.Collection, urlAndImage URLAndImage) {
	document := mongoDBImageDocument{
		BookURL: urlAndImage.URL,
		Image:   urlAndImage.Image,
	}
	_, err := coll.InsertOne(context.TODO(), document)
	if err != nil {
		fmt.Println("Error occurred while inserting image document in MongoDB", err)
	}
}
