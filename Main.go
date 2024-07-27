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

	if os.Args[1] == "--fullScrape" {
		FullScraping()
	}
	if os.Args[1] == "--repricer" {
		Repricer()
	}

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds.")
}
