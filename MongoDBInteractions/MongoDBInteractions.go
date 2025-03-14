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
var MondadoriProductsCollection *mongo.Collection
var feltrinelliBooksCollection *mongo.Collection
var MondadoriBooksCollection *mongo.Collection
var feltrinelliBooksToUpdateCollection *mongo.Collection
var mondadoriBooksToUpdateCollection *mongo.Collection
var BestsellersAmazonCollection *mongo.Collection
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

	//database := client.Database("Rimanga")
	database := client.Database("Mondadori")
	// Send a ping to confirm a successful connection

	if err = database.RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}

	FeltrinelliProductsCollection = database.Collection("FeltrinelliProducts")
	MondadoriProductsCollection = database.Collection("MondadoriProducts")
	feltrinelliBooksCollection = database.Collection("FeltrinelliBooks")
	MondadoriBooksCollection = database.Collection("MondadoriBooks")
	mondadoriBooksToUpdateCollection = database.Collection("BooksToUpdate")
	feltrinelliBooksToUpdateCollection = database.Collection("FeltrinelliBooksToUpdate")
	BestsellersAmazonCollection = database.Collection("BestsellersAmazon")
	ebayDataCollection = database.Collection("EbayData")
	ebayBooksCollection = database.Collection("EbayBooks")
	fmt.Println("Pinged mongodb deployment. Successfully connected to MongoDB!")
}

func GetAllEbayBooks() []DataTypes.EbayBook {
	cursor, _ := ebayBooksCollection.Find(context.TODO(), bson.M{})
	var ebayBooks []DataTypes.EbayBook
	err := cursor.All(context.TODO(), &ebayBooks)
	if err != nil {
		panic(err)
	}
	return ebayBooks
}

func AddBookToUpdate(isbn string) {
	filter := bson.M{"ISBN": isbn}
	update := bson.M{"$set": bson.M{"ISBN": isbn}}
	updateOptions := options.Update().SetUpsert(true)
	_, err := mondadoriBooksToUpdateCollection.UpdateOne(context.TODO(), filter, update, updateOptions)
	if err != nil {
		panic(err)
	}
}

func CreateUpsertModelForEbayBooks(ebayBook DataTypes.EbayBook) mongo.WriteModel {
	filter := bson.M{"ISBN": ebayBook.ISBN}
	update := bson.M{"$set": ebayBook}
	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
}

func UpsertEbayBooks(models []mongo.WriteModel) {
	if len(models) == 0 {
		return
	}
	_, err := ebayBooksCollection.BulkWrite(context.Background(), models)
	if err != nil {
		panic(err)
	}
}

func DeleteEbayBook(isbn string) {
	filter := bson.M{"ISBN": isbn}
	_, err := ebayBooksCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		panic(err)
	}
}

func UpsertEbayData(models []mongo.WriteModel) {
	if len(models) == 0 {
		return
	}
	_, err := ebayDataCollection.BulkWrite(context.Background(), models)
	if err != nil {
		panic(err)
	}
}

func GetEbayBook(isbn string) *DataTypes.EbayBook {
	filter := bson.M{"ISBN": isbn}
	var ebayBook DataTypes.EbayBook
	findOneError := ebayBooksCollection.FindOne(context.TODO(), filter).Decode(&ebayBook)
	if findOneError != nil {
		return nil
	}
	return &ebayBook
}
