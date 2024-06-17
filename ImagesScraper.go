package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/imroc/req/v3"
	"net/http"
	"strings"
	"time"
)

type URLAndImage struct {
	URL   string
	Image []byte
}

func downloadNewBookImages(books []FullBookInfo, imagesChannel chan<- URLAndImage) {
	fmt.Println("Started downloading images for new books")
	q, _ := queue.New(55, &queue.InMemoryQueueStorage{MaxSize: len(books)})
	for _, book := range books {
		err := q.AddURL(book.ImageURL)
		if err != nil {
			fmt.Println("Error occurred while adding an image URL to the queue.", book.URL, err)
		}
	}

	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)

	c.OnResponse(func(r *colly.Response) {
		imagesChannel <- URLAndImage{URL: r.Request.URL.String(), Image: r.Body}
	})

	c.OnError(func(response *colly.Response, err error) {
		if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host.") {
			errRetry := response.Request.Retry()
			if errRetry != nil {
				fmt.Println("Error occurred when retrying", errRetry)
			}
		}
		fmt.Println("Error occurred while trying to download image. URL is:", response.Request.URL.String(), err)
	})

	go func() {
		defer close(imagesChannel)
		err := q.Run(c)
		if err != nil {
			fmt.Println("Error running queue of tasks to download images")
			return
		}
		c.Wait()
		fmt.Println("Finished downloading images")
	}()
}
