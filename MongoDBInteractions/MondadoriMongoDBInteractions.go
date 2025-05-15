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
	fmt.Println("Removing all unseen Mondadori products.")
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
	for cursor.Next(context.TODO()) {
		var result DataTypes.MondadoriProduct
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
	_, err = mondadoriBooksCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}
	fmt.Println("Removed all unseen Mondadori books.")

	// Delete products with given URLs
	_, err = mondadoriProductsCollection.BulkWrite(context.TODO(), deleteModels, bulkOption)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred during bulk delete operation:", err)
		panic(err)
	}
	fmt.Println("Removed all unseen Mondadori products.")

	fmt.Println("Removed all unseen Mondadori products and books ", time.Since(startTime).Seconds(), "seconds.")
}

func BulkWriteMondadoriProducts(models []mongo.WriteModel) {
	bulkOptions := options.BulkWrite().SetOrdered(false)
	_, err := mondadoriProductsCollection.BulkWrite(context.TODO(), models, bulkOptions)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}
}

func CreateUpsertModelFromMondadoriBook(book DataTypes.MondadoriBook) *mongo.UpdateOneModel {
	filter := bson.M{"ISBN": book.ISBN}
	update := bson.M{
		"$set": bson.M{
			"ISBN":          book.ISBN,
			"Title":         book.Title,
			"Available":     book.Available,
			"Price":         book.Price,
			"ImageURL":      book.ImageURL,
			"Author":        book.Author,
			"Editor":        book.Editor,
			"Variant":       book.Variant,
			"Language":      book.Language,
			"PublishedFrom": book.PublishedFrom,
			"Pages":         book.Pages,
			"Description":   book.Description,
			"Series":        book.Series,
			"Categories":    book.Categories},
	}

	model := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(update).
		SetUpsert(true)
	return model
}

func UpsertMondadoriBooks(models []mongo.WriteModel) {
	bulkOptions := options.BulkWrite().SetOrdered(false)
	_, err := mondadoriBooksCollection.BulkWrite(context.TODO(), models, bulkOptions)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while upserting book documents in mondadoriBooksCollection", err)
		if err != nil {
			panic(err)
		}
	}
}

func UpsertMondadoriProducts(models []mongo.WriteModel) {
	bulkOptions := options.BulkWrite().SetOrdered(false)
	_, err := mondadoriProductsCollection.BulkWrite(context.TODO(), models, bulkOptions)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, "Error occurred while upserting in mondadoriProductsCollection", err)
		if err != nil {
			panic(err)
		}
	}
}

func CreateUpsertModelFromMondadoriBookForProducts(book DataTypes.MondadoriBook) *mongo.UpdateOneModel {
	filter := bson.M{"URL": book.URL}
	update := bson.M{
		"$set": bson.M{
			"ISBN": book.ISBN,
		},
	}

	model := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(update).
		SetUpsert(true)
	return model
}

func GetAllMondadoriURLs(urlsChan chan<- string) {
	opts := options.Find().SetBatchSize(1000)
	cursor, err := mondadoriProductsCollection.Find(context.TODO(), bson.M{}, opts)
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

	for cursor.Next(context.TODO()) {
		var result DataTypes.MondadoriProduct
		err = cursor.Decode(&result)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error occurred while decoding result", err)
			if err != nil {
				panic(err)
			}
		}
		urlsChan <- result.URL
	}

	if cursor.Err() != nil {
		_, err = fmt.Fprintln(os.Stderr, "Error occurred with cursor while iterating results", cursor.Err())
		if err != nil {
			panic(err)
		}
	}

	close(urlsChan)
}

func GetAllMondadoriURLsOnEbay(urlsChan chan<- string) {
	startTime := time.Now()

	// Define the aggregation pipeline
	pipeline := bson.A{
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "EbayData", Value: "$$ROOT"},
			}},
		},
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "MondadoriProducts"},
				{Key: "localField", Value: "EbayData.ISBN"},
				{Key: "foreignField", Value: "ISBN"},
				{Key: "as", Value: "MondadoriProduct"},
			}},
		},
		bson.D{
			{Key: "$unwind", Value: bson.D{
				{Key: "path", Value: "$MondadoriProduct"},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "MondadoriProduct.URL", Value: 1},
			}},
		},
		bson.D{
			{Key: "$replaceRoot", Value: bson.D{
				{Key: "newRoot", Value: "$MondadoriProduct"},
			}},
		},
	}

	// Execute the aggregation pipeline
	cursor, err := ebayDataCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {

		}
	}(cursor, context.TODO())

	// Iterate through the results
	var results []DataTypes.URLDocument
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Finished getting all URLs in", time.Since(startTime).Seconds(), "seconds")
	for _, result := range results {
		urlsChan <- result.URL
	}
	close(urlsChan)
}

func GetMondadoriBook(isbn string) (*DataTypes.MondadoriBook, error) {
	filter := bson.M{"ISBN": isbn}
	var result DataTypes.MondadoriBook
	err := mondadoriBooksCollection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetAllMondadoriBooksOnEbay() map[string]DataTypes.EbayDataWithMondadoriBook {
	startTime := time.Now()

	// Define the aggregation pipeline
	pipeline := bson.A{
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "EbayData", Value: "$$ROOT"},
			}},
		},
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "MondadoriBooks"},
				{Key: "localField", Value: "EbayData.ISBN"},
				{Key: "foreignField", Value: "ISBN"},
				{Key: "as", Value: "MondadoriBook"},
			}},
		},
		bson.D{
			{Key: "$unwind", Value: bson.D{
				{Key: "path", Value: "$MondadoriBook"},
			}},
		},
	}

	// Execute the aggregation pipeline
	cursor, err := ebayDataCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		log.Fatal(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {

		}
	}(cursor, context.TODO())

	// Iterate through the results
	var results []DataTypes.EbayDataWithMondadoriBook
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Finished getting all Mondadori books on Ebay in", time.Since(startTime).Seconds(), "seconds")

	outputMap := make(map[string]DataTypes.EbayDataWithMondadoriBook, len(results))
	for _, result := range results {
		outputMap[result.MondadoriBook.ISBN] = result
	}
	return outputMap
}

func BuildMondadoriProductUpsertModel(entry DataTypes.MondadoriSitemapItem, lastSeen time.Time) *mongo.UpdateOneModel {
	filter := bson.M{"URL": entry.Loc}

	update := bson.M{
		"$set": bson.M{
			"URL":      entry.Loc,
			"LastSeen": lastSeen,
		},
	}

	model := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(update).
		SetUpsert(true)
	return model
}

func UpsertMondadoriBooksAndProducts(models []mongo.WriteModel, models2 []mongo.WriteModel) {
	startTime := time.Now()
	fmt.Println("Upserting", len(models), "Mondadori books and products.")

	UpsertMondadoriBooks(models)
	UpsertMondadoriProducts(models2)

	fmt.Println("Upserting took", time.Since(startTime).Seconds(), "seconds.")
}

func DeleteMondadoriBook(URL string) {
	filter := bson.M{"URL": URL}
	result, err := mondadoriBooksCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		panic(err)
	}
	if result != nil && result.DeletedCount < 1 {
		panic("Mondadori book was not deleted. URL:" + URL)
	}
}

func DeleteMondadoriProduct(URL string) {
	filter := bson.M{"URL": URL}
	result, err := mondadoriProductsCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		panic(err)
	}
	if result != nil && result.DeletedCount < 1 {
		panic("Mondadori product was not deleted. URL:" + URL)
	}
}

func GetMondadoriProduct(URL string) (*DataTypes.MondadoriProduct, error) {
	filter := bson.M{"URL": URL}
	var result DataTypes.MondadoriProduct
	err := mondadoriProductsCollection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
