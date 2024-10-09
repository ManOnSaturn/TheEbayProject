package main

import (
	"fmt"
	"os"
	"time"
)

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

	if len(os.Args) > 1 && os.Args[1] == "--scrapeBestsellers" {
		ScrapeBestsellers()
	}

	if len(os.Args) > 1 && os.Args[1] == "--fullScrape" {
		FullScraping()
	}

	if len(os.Args) > 1 && os.Args[1] == "--repricer" {
		Repricer()
	}

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}
