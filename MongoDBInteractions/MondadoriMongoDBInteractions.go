package MongoDBInteractions

import (
	"Scraper/DataTypes"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"time"
)

func RemoveAllUnseenProductsAndBooksMondadori(lastSeen time.Time) {
	fmt.Println("Removing all unseen products")
	startTime := time.Now()
	filter := bson.M{
		"LastSeen": bson.M{
			"$ne": lastSeen,
		},
	}

	cursor, err := mondadoriProductsCollection.Find(context.TODO(), filter)
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
	mondadoriBooksISBNs := make([]string, 0)
	for cursor.Next(context.TODO()) {
		var result DataTypes.MongoDBMondadoriProduct
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
		var mondadoriBook DataTypes.MondadoriBook
		findOneError := mondadoriBooksCollection.FindOne(context.TODO(), bson.M{"URL": result.URL}).Decode(&mondadoriBook)
		if findOneError == nil {
			mondadoriBooksISBNs = append(mondadoriBooksISBNs, mondadoriBook.ISBN)
		}
	}

	if len(deleteModels) == 0 {
		return
	}

	fmt.Printf("Removing %d products.\n", len(deleteModels))

	bulkOption := options.BulkWrite().SetOrdered(false)

	// Delete products with given URLs
	_, err = mondadoriProductsCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Delete books with given URLs
	_, err = mondadoriBooksCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}

	// Change availability if book is published
	var booksToUpdateModels []mongo.WriteModel
	for _, ISBN := range mondadoriBooksISBNs {
		documents, _ := ebayDataCollection.CountDocuments(context.TODO(), bson.M{"ISBN": ISBN})
		if documents > 0 {
			model := mongo.NewUpdateOneModel()
			model.SetFilter(bson.M{"ISBN": ISBN})
			model.SetUpsert(true)
			model.SetUpdate(bson.M{"$set": bson.M{"ISBN": ISBN, "Available": "Rimosso", "AvailabilityChanged": true, "Price": "", "PriceChanged": false}})
			booksToUpdateModels = append(booksToUpdateModels, model)
		}
	}

	// TODO reason about whether this is actually needed, or needs to be checked during repricing, or both
	if len(booksToUpdateModels) > 0 {
		fmt.Println("Storing books which disappeared as books to update")
		_, err = mondadoriBooksToUpdateCollection.BulkWrite(context.TODO(), booksToUpdateModels)
		if err != nil {
			_, err = fmt.Fprintln(os.Stderr, "Error occurred during bulk write operation:", err)
			return
		}
	}

	fmt.Println("Removed all unseen products in ", time.Since(startTime).Seconds(), "seconds.")
}

func BulkWriteMondadoriProducts(models []mongo.WriteModel) {
	_, err := mondadoriProductsCollection.BulkWrite(context.TODO(), models)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}
