package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
var feltrinelliProductsCollection *mongo.Collection
var mondadoriProductsCollection *mongo.Collection
var feltrinelliBooksCollection *mongo.Collection
var mondadoriBooksCollection *mongo.Collection
var booksToUpdateCollection *mongo.Collection
var bestsellersAmazonCollection *mongo.Collection
var ebayDataCollection *mongo.Collection
var ebayBooksCollection *mongo.Collection

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

	database := client.Database("Mondadori")
	// Send a ping to confirm a successful connection

	if err = database.RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}

	feltrinelliProductsCollection = database.Collection("FeltrinelliProducts")
	mondadoriProductsCollection = database.Collection("MondadoriProducts")
	feltrinelliBooksCollection = database.Collection("FeltrinelliBooks")
	mondadoriBooksCollection = database.Collection("MondadoriBooks")
	booksToUpdateCollection = database.Collection("BooksToUpdate")
	bestsellersAmazonCollection = database.Collection("BestsellersAmazon")
	ebayDataCollection = database.Collection("EbayData")
	ebayBooksCollection = database.Collection("EbayBooks")
	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
}

func AddBookToUpdate(isbn string) {
	filter := bson.M{"ISBN": isbn}
	update := bson.M{"$set": bson.M{"ISBN": isbn}}
	updateOptions := options.Update().SetUpsert(true)
	_, err := booksToUpdateCollection.UpdateOne(context.TODO(), filter, update, updateOptions)
	if err != nil {
		panic(err)
	}
}

type Publisher string

const (
	Mondadori   Publisher = "Mondadori"
	Feltrinelli Publisher = "Feltrinelli"
)

func RemoveAllUnseenProductsAndBooks(lastSeen time.Time, publisher Publisher) {
	var productsCollection *mongo.Collection
	var booksCollection *mongo.Collection
	switch publisher {
	case Mondadori:
		productsCollection = mondadoriProductsCollection
		booksCollection = mondadoriBooksCollection
	case Feltrinelli:
		productsCollection = feltrinelliProductsCollection
		booksCollection = feltrinelliBooksCollection
	default:
		panic("Unsupported publisher")
	}

	fmt.Println("Removing all unseen", publisher, "products.")
	startTime := time.Now()
	filter := bson.M{
		"LastSeen": bson.M{
			"$ne": lastSeen,
		},
	}

	cursor, err := productsCollection.Find(context.TODO(), filter)
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
		// This can be either MondadoriProduct or FeltrinelliProduct.
		var result DataTypes.URLDocument
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
	_, err = booksCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Delete products with given URLs
	_, err = productsCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	fmt.Println("Removed all unseen", publisher, "products and books in", time.Since(startTime).Seconds(), "seconds.")
}
