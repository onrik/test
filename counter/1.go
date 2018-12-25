package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type WordsCounter func(url, word string) (int, error)

type Result struct {
	Url   string
	Count int
}

type Pool struct {
	maxWorkers int
	wg         sync.WaitGroup
	urls       chan string
	results    chan Result
}

func NewPool(maxWorkers int) *Pool {
	return &Pool{
		maxWorkers: maxWorkers,
		wg:         sync.WaitGroup{},
		urls:       make(chan string),
		results:    make(chan Result),
	}
}

func (pool *Pool) Run(jobs <-chan string) {
	i := 0
	for url := range jobs {
		if url == "" {
			continue
		}

		if i <= pool.maxWorkers {
			i++
			pool.wg.Add(1)
			go pool.worker(countWords)
		}
		pool.urls <- url
	}
	close(pool.urls)
	pool.wg.Wait()
	close(pool.results)
}

func (pool *Pool) Results() <-chan Result {
	return pool.results
}

func (pool *Pool) worker(handler WordsCounter) {
	defer pool.wg.Done()

	for {
		url, ok := <-pool.urls
		if !ok {
			break
		}
		count, err := handler(url, "Go")
		if err != nil {
			log.Println(url, err)
			continue
		}
		pool.results <- Result{
			Url:   url,
			Count: count,
		}
	}
}

func countWords(url, word string) (int, error) {
	response, err := http.Get(url)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return 0, err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}

	return bytes.Count(data, []byte(word)), nil
}

func startReader(source io.Reader, channel chan string) {
	defer close(channel)
	reader := bufio.NewReader(source)
	for {
		url, err := reader.ReadString('\n')
		channel <- strings.TrimSpace(url)

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
	}
}

func main() {
	k := flag.Int("k", 5, "Count of workers")
	flag.Parse()

	urls := make(chan string)

	// Start workers pool
	pool := NewPool(*k)
	go pool.Run(urls)

	// Start reader
	go startReader(os.Stdin, urls)

	// Handle results
	total := 0
	for result := range pool.Results() {
		total += result.Count
		fmt.Printf("Count for %s: %d\n", result.Url, result.Count)
	}
	fmt.Println("Total:", total)
}
