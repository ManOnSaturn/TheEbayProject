package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/imroc/req/v3"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type MondadoriPageInfo struct {
	URL         string
	PageNumbers int
}

func getBooks(booksChannel chan<- BookFull) {
	var err error
	pageInfos := []MondadoriPageInfo{
		{URL: "https://www.mondadoristore.it/libri/italiani/Ambiente-e-Animali/genG001/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Architettura-Design-e-Moda/genG002/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Arte-Beni-culturali-e-Fotografia/genG003/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Bambini-e-Ragazzi/genG004/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Cinema-e-Spettacolo/genG005/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Economia-Diritto-e-Lavoro/genG006/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Esoterismo-e-Astrologia/genG007/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Famiglia-Scuola-e-Universita/genG008/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Fantasy-Horror-e-Gothic/genG009/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Gastronomia/genG00B/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Gialli-Noir-e-Avventura/genG00C/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Guide-turistiche-e-Viaggi/genG00D/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Hobby-e-Tempo-libero/genG00E/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Informatica-e-Web/genG00F/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Lingue-e-Dizionari/genG00G/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Musica/genG00H/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Passione-e-Sentimenti/genG00I/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Politica-e-Societa/genG00J/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Psicologia-e-Filosofia/genG00K/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Religioni-e-Spiritualita/genG00L/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Romanzi-e-Letterature/genG00M/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Salute-Benessere-Self-Help/genG00N/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Scienza-e-Tecnica/genG00O/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Sport/genG00P/"},
		{URL: "https://www.mondadoristore.it/libri/italiani/Storia-e-Biografie/genG00Q/"},
	}

	// Create colly collector, while impersonating chrome.
	fakeChrome := req.DefaultClient().ImpersonateChrome()
	c := colly.NewCollector(colly.AllowURLRevisit(), colly.UserAgent(fakeChrome.Headers.Get("user-agent")))
	c.SetClient(&http.Client{
		Transport: fakeChrome.Transport,
		Timeout:   30 * time.Second,
	})
	c.SetRequestTimeout(30 * time.Second)

	var URLList []string
	c.OnHTML("li.item.word", func(element *colly.HTMLElement) {
		if element.Index == 1 {
			nPages, _ := strconv.Atoi(element.Text)
			for nPage := range nPages {
				URLList = append(URLList, element.Request.URL.String()+strconv.Itoa(nPage+1)+"/")
			}
		}
	})

	for _, pageInfo := range pageInfos {
		err = c.Visit(pageInfo.URL)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error on visit:", err, "URL: ", pageInfo.URL)
			if err != nil {
				panic(err)
			}
		}
	}

	// Wait for the number of pages being computed before exploring all the content.
	c.Wait()
	// Remove previous "onHTML" listener.
	c.OnHTMLDetach("li.item.word")

	// Core logic: visit book pages and gather book details.
	pageVisitedChan := make(chan struct{}, 10)
	c.OnHTML("#div_container", func(element *colly.HTMLElement) {
		pageVisitedChan <- struct{}{}
		element.ForEach("div.single-box", func(index int, element *colly.HTMLElement) {
			bookURL := element.ChildAttr("a.link", "href")
			bookAvailableText := element.ChildText("span.time")
			bookAvailable := bookAvailableText == "Disponibilità immediata"
			bookISBN := element.ChildAttr("div.info-data-product", "data-dimension8")
			bookTitle := element.ChildAttr("div.info-data-product", "data-name")
			bookPrice := element.ChildAttr("div.info-data-product", "data-metric3")
			bookCategory := element.ChildAttr("div.info-data-product", "data-category")
			bookAuthor := element.ChildAttr("div.info-data-product", "data-dimension15")
			bookLanguage := element.ChildAttr("div.info-data-product", "data-dimension14")
			bookVariant := element.ChildAttr("div.info-data-product", "data-variant")
			bookEditor := element.ChildAttr("div.info-data-product", "data-brand")
			bookImageURL := element.ChildAttr("img.image.first-img.product-img.is-book.maxHeightLarge", "src")
			booksChannel <- BookFull{
				ISBN:      bookISBN,
				Title:     bookTitle,
				Available: bookAvailable,
				Price:     bookPrice,
				URL:       bookURL,
				ImageURL:  strings.Replace("https://www.mondadoristore.it"+bookImageURL, "/ZOM/", "/NZO/", 1),
				Author:    bookAuthor,
				Category:  bookCategory,
				Variant:   bookVariant,
				Editor:    bookEditor,
				Language:  bookLanguage,
			}
		})
	})

	// Retry (infinitely!) on page errors.
	pageErroredChan := make(chan struct{}, 5)
	c.OnError(func(response *colly.Response, err error) {
		pageErroredChan <- struct{}{}
		_ = response.Request.Retry()
	})

	// Wait for errored pages and log timing.
	numPageErrored := 0
	lastTimeError := time.Now()
	go func() {
		for range pageErroredChan {
			numPageErrored++
			if numPageErrored%1 == 0 {
				fmt.Println("1 error every", time.Now().Sub(lastTimeError))
				lastTimeError = time.Now()
			}
		}
	}()

	const maxQueueSize = 40000
	q, _ := queue.New(60, &queue.InMemoryQueueStorage{MaxSize: maxQueueSize})

	// Shuffling for the sake of not visiting all the pages from the same category, to average their speeds.
	shuffle(URLList)
	// Adding all URLs to the queue
	for _, URL := range URLList {
		err := q.AddURL(URL)
		if err != nil {
			_, err := fmt.Fprintln(os.Stderr, "Error on adding URL:", err)
			if err != nil {
				panic(err)
			}
		}
	}

	queueSize, _ := q.Size()

	go logVisitedPages(pageVisitedChan, queueSize)

	fmt.Println("Running queue of request of size: ", queueSize)
	if queueSize > maxQueueSize-1000 {
		_, err := fmt.Fprintln(os.Stderr, "Increase running queue size before it's too late!")
		if err != nil {
			fmt.Println("Error on using Fprintln on stderr:", err)
		}
	}
	err = q.Run(c) // Blocking
	if err != nil {
		panic(err)
	}

	c.Wait()
}

func logVisitedPages(pageVisitedChan chan struct{}, queueSize int) {
	numPageVisited := 0
	lastPageVisited := 0
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case _, ok := <-pageVisitedChan:
			if !ok {
				return
			}
			numPageVisited++
		case <-ticker.C:
			fmt.Println(numPageVisited-lastPageVisited, "/s.", numPageVisited, "/", queueSize, (numPageVisited/queueSize)*100, "%")
			lastPageVisited = numPageVisited
		}
	}
}

// Shuffle shuffles the elements of a slice
func shuffle(slice []string) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}
