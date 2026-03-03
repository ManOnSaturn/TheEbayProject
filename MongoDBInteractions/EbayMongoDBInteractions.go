package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetAllEbayBooks() []DataTypes.EbayBook {
	cursor, _ := ebayBooksCollection.Find(context.TODO(), bson.M{})
	var ebayBooks []DataTypes.EbayBook
	err := cursor.All(context.TODO(), &ebayBooks)
	if err != nil {
		panic(err)
	}
	return ebayBooks
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

func DeleteEbayData(isbn string) {
	filter := bson.M{"ISBN": isbn}
	_, err := ebayDataCollection.DeleteOne(context.Background(), filter)
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

func GetEbayData(isbn string) *DataTypes.EbayData {
	filter := bson.M{"ISBN": isbn}
	var ebayData DataTypes.EbayData
	findOneError := ebayDataCollection.FindOne(context.TODO(), filter).Decode(&ebayData)
	if findOneError != nil {
		return nil
	}
	return &ebayData
}
