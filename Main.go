package main

import (
	"fmt"
	"sync"
	"time"
)

type PageInfo struct {
	URL         string
	PageNumbers int
}

type PartialBookInfo struct {
	Price     string
	Available bool
}

type FullBookInfo struct {
	URL       string
	Price     string
	Available bool
	ISBN      string
	Title     string
	Author    string
	Category  string
	Language  string
	Editor    string
	ImageURL  string
}

func main() {

	bookInfoChannel := make(chan FullBookInfo, 500)
	dbBookInfos := make(map[string]PartialBookInfo, 750000)
	var wg sync.WaitGroup

	startTime := time.Now()

	go func() {
		defer wg.Done()
		wg.Add(1)
		getBooks(bookInfoChannel)
	}()

	go func() {
		wg.Wait()
		close(bookInfoChannel)
	}()

	bookInfosMap := make(map[FullBookInfo]bool, 750000)

Loop:
	for {
		select {
		// Wait for all URLs to be gathered before closing the channel
		case bookInfo, ok := <-bookInfoChannel:
			if !ok {
				break Loop
			}
			bookInfosMap[bookInfo] = true
		}
	}
	fmt.Println("Finished scraping book infos in ", time.Since(startTime).Seconds(), "seconds")

	var booksToAdd []FullBookInfo
	booksToUpdate := make(map[string]PartialBookInfo, 100)

	fmt.Println("Creating lists of books to create and books to update")
	for newBookInfo := range bookInfosMap {
		bookFromDB, ok := dbBookInfos[newBookInfo.URL]
		if !ok {
			booksToAdd = append(booksToAdd, newBookInfo)
		} else {
			if bookFromDB.Available != newBookInfo.Available || bookFromDB.Price != newBookInfo.Price {
				booksToUpdate[newBookInfo.URL] = PartialBookInfo{Price: newBookInfo.Price, Available: newBookInfo.Available}
			}
		}
	}
	fmt.Println(len(booksToAdd), "books to add.", len(booksToUpdate), "books to update.")

	imageURLToBookURLMap := make(map[string]string, len(booksToAdd))
	for _, bookToAdd := range booksToAdd {
		imageURLToBookURLMap[bookToAdd.ImageURL] = bookToAdd.URL
	}

	imagesChannel := make(chan URLAndImage, 500)

	downloadNewBookImages(booksToAdd, imagesChannel)

	client, err := connectToMongo("mongodb://localhost:27017/")
	if err != nil {
		fmt.Println("Error occurred while trying to connect to MongoDB")
		return
	}

	booksCollection := client.Database("Mondadori").Collection("Books")
	imagesCollection := client.Database("Mondadori").Collection("BookImages")

	var mongoDBOperationsWG sync.WaitGroup

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		for urlAndImage := range imagesChannel {
			urlAndImage.URL = imageURLToBookURLMap[urlAndImage.URL]
			insertImage(imagesCollection, urlAndImage)
		}
		fmt.Println("Finished inserting images in DB")
	}()

	mongoDBOperationsWG.Add(1)
	go func() {
		defer mongoDBOperationsWG.Done()
		for _, bookToAdd := range booksToAdd {
			insertBook(booksCollection, bookToAdd)
		}
		fmt.Println("Finished inserting books in DB")
	}()

	mongoDBOperationsWG.Wait()

	// TODO Update infos

	fmt.Println("Finished running in", time.Since(startTime).Seconds(), "seconds")

}
