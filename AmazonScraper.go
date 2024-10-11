package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/imroc/req/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// https://www.amazon.it/gp/bestsellers/books

type Item struct {
	ID string `json:"id"`
}

func getPrunedASINs(client *mongo.Client, ASINs []string) []string {
	prunedASINs := make([]string, 0)
	bestsellersCollection := client.Database("Mondadori").Collection("BestsellersAmazon")
	for _, ASIN := range ASINs {
		count, err := bestsellersCollection.CountDocuments(context.TODO(), bson.D{{"ASIN", ASIN}})
		if err != nil {
			break
		}

		if count == 0 {
			prunedASINs = append(prunedASINs, ASIN)
		}
	}
	return prunedASINs
}

func scrapeBestsellers() {
	asins := getASINs()
	client := connectToMongo()
	prunedASINs := getPrunedASINs(client, asins)
	ASINISBNPairs := getISBNs(prunedASINs)
	bestsellersCollection := client.Database("Mondadori").Collection("BestsellersAmazon")

	var bulkOps []mongo.WriteModel
	for _, pair := range ASINISBNPairs {
		filter := bson.D{{"ISBN", pair.ISBN}}
		update := bson.D{
			{"$set", bson.D{
				{"ASIN", pair.ASIN},
				{"ISBN", pair.ISBN}}},
		}
		bulkOps = append(bulkOps, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Execute the bulk write
	result, err := bestsellersCollection.BulkWrite(ctx, bulkOps)
	if err != nil {
		log.Fatalf("Failed to execute bulk write: %v", err)
	}

	fmt.Printf("Upserted %d documents and modified %d documents.\n", result.UpsertedCount, result.ModifiedCount)
}

type ASINISBNPair struct {
	ASIN string
	ISBN string
}

func getISBNs(asins []string) []ASINISBNPair {
	fakeChrome := req.DefaultClient().ImpersonateChrome()

	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)
	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1 * time.Second,
	})
	if err != nil {
		panic(err)
	}

	ASINISBNPairs := make([]ASINISBNPair, 0)
	isbnRegex := regexp.MustCompile(`\d{3}-\d{10}`)

	c.OnHTML("body", func(element *colly.HTMLElement) {
		isbn := isbnRegex.FindString(element.Text)
		asin := element.Request.URL.Path[4:]
		if len(isbn) < 13 {
			_, err := fmt.Fprintln(os.Stderr, "ISBN shorter than 13 characters:", isbn, "For ASIN:", asin)
			if err != nil {
				panic(err)
			}
			return
		}
		ASINISBNPairs = append(ASINISBNPairs, ASINISBNPair{ASIN: asin, ISBN: strings.Replace(isbn, "-", "", 1)})
	})

	c.OnError(func(response *colly.Response, err error) {
		println(response.Request.URL)
		println(err.Error())
	})

	for _, asin := range asins {
		err := c.Visit("https://www.amazon.it/dp/" + asin)
		if err != nil {
			println(err)
		}
	}

	c.Wait()
	return ASINISBNPairs
}

func getASINs() []string {
	URLs := []string{
		"https://www.amazon.it/gp/bestsellers/books",
		"https://www.amazon.it/gp/bestsellers/books/13077484031", // Adolescenti e ragazzi
		"https://www.amazon.it/gp/bestsellers/books/508714031",   // Biografie
		"https://www.amazon.it/gp/bestsellers/books/508864031",   // Dizionari e opere di consultazione
		"https://www.amazon.it/gp/bestsellers/books/508785031",   // Diritto
		"https://www.amazon.it/gp/bestsellers/books/508786031",   // Economia, affari e finanza
		"https://www.amazon.it/gp/bestsellers/books/508773031",   // Fantascienza
		"https://www.amazon.it/gp/bestsellers/books/508772031",   // Fantasy
		"https://www.amazon.it/gp/bestsellers/books/508784031",   // Fumetti e Manga
		"https://www.amazon.it/gp/bestsellers/books/508771031",   // Gialli e Thriller
		"https://www.amazon.it/gp/bestsellers/books/508821031",   // Tempo libero
		"https://www.amazon.it/gp/bestsellers/books/508774031",   // Horror
		"https://www.amazon.it/gp/bestsellers/books/508820031",   // Humour
		"https://www.amazon.it/gp/bestsellers/books/508733031",   // Informatica, Web e digital media
		"https://www.amazon.it/gp/bestsellers/books/13466598031", // Lettura erotica
		"https://www.amazon.it/gp/bestsellers/books/508770031",   // Letteratura e narrativa
		"https://www.amazon.it/gp/bestsellers/books/508811031",   // Politica
		"https://www.amazon.it/gp/bestsellers/books/508745031",   // Religione
		"https://www.amazon.it/gp/bestsellers/books/508775031",   // Romanzi rosa
		"https://www.amazon.it/gp/bestsellers/books/508792031",   // Famiglia salute e benessere
		"https://www.amazon.it/gp/bestsellers/books/508867031",   // Scienza, tecnologia e medicina
		"https://www.amazon.it/gp/bestsellers/books/508794031",   // Self-help
		"https://www.amazon.it/gp/bestsellers/books/508879031",   // Società e scienze sociali
		"https://www.amazon.it/gp/bestsellers/books/508835031",   // Sport
		"https://www.amazon.it/gp/bestsellers/books/508796031",   // Storia
		"https://www.amazon.it/gp/bestsellers/books/508753031",   // Viaggi
	}
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)
	links := make([]string, 0)

	c.OnHTML("div.p13n-desktop-grid", func(element *colly.HTMLElement) {
		links = append(links, element.Attr("data-client-recs-list"))
	})

	c.OnHTML("ul.a-pagination li.a-normal a", func(element *colly.HTMLElement) {
		secondPageLink := element.Attr("href")
		if !strings.HasSuffix(secondPageLink, "pg=1") {
			err := c.Visit("https://www.amazon.it" + secondPageLink)
			if err != nil {
				return
			}
		}
	})

	c.OnError(func(response *colly.Response, err error) {
		println(response)
		println(err)
	})

	for _, URL := range URLs {
		err := c.Visit(URL)
		if err != nil {
			println(err)

		}
	}

	c.Wait()

	var itemArrays [][]Item
	for _, link := range links {
		var items []Item
		err := json.Unmarshal([]byte(link), &items)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON for link %s: %v", link, err)
		}
		itemArrays = append(itemArrays, items)
	}

	// Collect the IDs
	ASINs := make(map[string]struct{})
	for _, itemArray := range itemArrays {
		for _, item := range itemArray {
			ASINs[item.ID] = struct{}{}
		}
	}

	// Convert map keys to a slice
	ASINList := make([]string, 0, len(ASINs))
	for ASIN := range ASINs {
		ASINList = append(ASINList, ASIN)
	}
	return ASINList
}
