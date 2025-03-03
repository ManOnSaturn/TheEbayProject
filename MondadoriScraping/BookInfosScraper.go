package MondadoriScraping

import (
	"fmt"
	"os"
	"time"
)

type MondadoriPageInfo struct {
	URL         string
	PageNumbers int
}

func logErroredPages(pageErroredChan chan struct{}) {
	numPageErrored := 0
	lastTimeError := time.Now()
	for range pageErroredChan {
		numPageErrored++
		fmt.Println("Error every", time.Now().Sub(lastTimeError))
		lastTimeError = time.Now()
		if numPageErrored > 1000 {
			_, err := fmt.Fprintln(os.Stderr, "1000 errors reached. Closing.")
			if err != nil {
				panic(err)
			}
			os.Exit(-1)
		}
	}
	fmt.Println(numPageErrored, "errored pages.")
}

func logVisitedPages(pageVisitedChan chan struct{}, queueSize int) {
	numPageVisited := 0
	lastPageVisited := 0
	seconds := 5
	ticker := time.NewTicker(time.Duration(seconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case _, ok := <-pageVisitedChan:
			if !ok {
				fmt.Printf("Pages visited %d/%d.\n", numPageVisited, queueSize)
				return
			}
			numPageVisited++
		case <-ticker.C:
			percentage := (float64(numPageVisited) / float64(queueSize)) * 100
			fmt.Printf("%d/%ds. %d/%d (%.2f%%)\n", numPageVisited-lastPageVisited, seconds, numPageVisited, queueSize, percentage)
			lastPageVisited = numPageVisited
		}
	}
}
