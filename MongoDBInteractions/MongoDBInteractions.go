package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"runtime"
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

	//database := client.Database("Rimanga")
	database := client.Database("Mondadori")
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

func Reschema() {
	cursor, _ := mondadoriBooksCollection.Find(context.TODO(), bson.M{"ListingId": bson.M{"$exists": true}})
	var models []mongo.WriteModel
	for cursor.Next(context.TODO()) {
		var result DataTypes.MondadoriBookDocument
		err := cursor.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}

		filter := bson.M{"ISBN": result.ISBN}
		document := DataTypes.EbayData{
			ISBN:           result.ISBN,
			PublishedPrice: result.PublishedPrice,
			Published:      result.Published,
			ListingId:      result.ListingId,
			OfferId:        result.OfferId,
			EbayImageURL:   result.EbayImageUrl,
			MarketIn:       "Mondadori",
		}
		update := bson.M{"$set": document}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}

	_, err := ebayDataCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		panic(err)
	}
}
