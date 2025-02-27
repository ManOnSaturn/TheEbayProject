package main

import (
	"Scraper/AmazonScraping"
	"Scraper/FeltrinelliScraping"
	"Scraper/MondadoriScraping"
	"Scraper/MongoDBInteractions"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)
import _ "net/http/pprof"

func main() {
	startTime := time.Now()
	MongoDBInteractions.ConnectToMongo()
	defer MongoDBInteractions.DisconnectFromMongo()

	if len(os.Args) > 1 && os.Args[1] == "--scrapeBestsellers" {
		AmazonScraping.ScrapeBestsellers()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrape" {
		MondadoriScraping.FullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--repricer" {
		MondadoriScraping.Repricer()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeFeltrinelli" {
		go func() {
			log.Println(http.ListenAndServe("0.0.0.0:8888", nil))
			panic("what")
		}()
		FeltrinelliScraping.FullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--feltrinelliRepricer" {
		go func() {
			log.Println(http.ListenAndServe("0.0.0.0:8888", nil))
			panic("what")
		}()
		FeltrinelliScraping.Repricer()
	}

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}
