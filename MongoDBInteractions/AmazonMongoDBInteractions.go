package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func IsASINStored(ASIN string) bool {
	count, err := bestsellersAmazonCollection.CountDocuments(context.TODO(), bson.D{{"ASIN", ASIN}})
	if err != nil {
		panic(err)
	}
	return count > 0
}

func BulkWriteBestsellers(bulkOps []mongo.WriteModel) *mongo.BulkWriteResult {
	result, err := bestsellersAmazonCollection.BulkWrite(context.TODO(), bulkOps)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
	return result
}

func BuildAmazonBestsellerIsKindleUpdateModel(asin string) *mongo.UpdateOneModel {
	filter := bson.D{{"ASIN", asin}}
	update := bson.D{
		{"$set", bson.D{
			{"ASIN", asin},
			{"isKindle", true}}},
	}
	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
}

func BuildAmazonBestsellerISBNUpdateModel(pair DataTypes.ASINISBNPair) *mongo.UpdateOneModel {
	filter := bson.D{{"ISBN", pair.ISBN}}
	update := bson.D{
		{"$set", bson.D{
			{"ASIN", pair.ASIN},
			{"ISBN", pair.ISBN}}},
	}
	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
}
