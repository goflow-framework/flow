package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8081/users/123/profile", "target URL")
	concurrency := flag.Int("c", 50, "concurrency")
	duration := flag.Duration("d", 15*time.Second, "duration")
	flag.Parse()

	stop := time.Now().Add(*duration)
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				resp, err := client.Get(*url)
				if err == nil && resp.Body != nil {
					resp.Body.Close()
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("done")
}
