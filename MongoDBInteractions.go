package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoDBBookDocument struct {
	URL       string `bson:"URL"`
	ImageURL  string `bson:"ImageURL"`
	Price     string `bson:"Price"`
	Available bool   `bson:"Available"`
	ISBN      string `bson:"ISBN"`
	Title     string `bson:"Title"`
	Author    string `bson:"Author"`
	Category  string `bson:"Category"`
	Language  string `bson:"Language"`
	Variant   string `bson:"Variant"`
	Editor    string `bson:"Editor"`
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

func insertBook(coll *mongo.Collection, book FullBookInfo) {
	//document := mongoDBBookDocument{
	//	URL:       book.URL,
	//	ImageURL:  book.ImageURL,
	//	Price:     book.Price,
	//	Available: book.Available,
	//	ISBN:      book.ISBN,
	//	Title:     book.Title,
	//	Author:    book.Author,
	//	Category:  book.Category,
	//	Language:  book.Language,
	//	Variant:   book.Variant,
	//	Editor:    book.Editor,
	//}
	//_, err := coll.InsertOne(context.TODO(), document)
	//if err != nil {
	//	fmt.Println("Error occurred while inserting book document in MongoDB", err)
	//}
}
