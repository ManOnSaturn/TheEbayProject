package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)
import _ "net/http/pprof"

type BookPartial struct {
	ISBN      string
	Price     string
	Available string
}

type BookToUpdate struct {
	ISBN                string
	Price               string
	PriceChanged        bool
	Available           string
	AvailabilityChanged bool
}

type BookFull struct {
	ISBN      string
	Published bool
	Title     string
	Available string
	Price     string
	URL       string
	ImageURL  string
	Author    string
	Category  string
	Variant   string
	Editor    string
	Language  string
}

func main() {
	startTime := time.Now()
	connectToMongo()
	defer disconnectFromMongo()
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:8888", nil))
		panic("what")
	}()

	if len(os.Args) > 1 && os.Args[1] == "--scrapeBestsellers" {
		scrapeBestsellers()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrape" {
		fullScrape()
	}

	if len(os.Args) > 1 && os.Args[1] == "--repricer" {
		repricer()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrapeFeltrinelli" {
		fullScrapeFeltrinelli()
	}

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}
