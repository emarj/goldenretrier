package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

/* target, err := url.Parse(targetURL)
if err != nil {
	return fmt.Errorf("Error parsing target URL:", err)
} */

const capacity = 15

type Retrier struct {
	queue    chan *http.Request
	capacity int
	timeout  time.Duration
	target   *url.URL
}

func NewRetrier(targetURL string) (*Retrier, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	return &Retrier{
		capacity: capacity,
		queue:    make(chan *http.Request, 2*capacity),
		timeout:  10 * time.Second,
		target:   u,
	}, nil
}

func (ret *Retrier) Add(req *http.Request) error {

	if len(ret.queue)*2 == cap(ret.queue) {
		return fmt.Errorf("the queue is full")
	}

	ret.queue <- req
	return nil
}

func (ret *Retrier) Start() error {
	ticker := time.NewTicker(time.Second * 3)
	go func() {
		for {
			<-ticker.C
			ret.Retry()
		}
	}()

	return nil
}

func (ret *Retrier) Retry() {
	n := len(ret.queue)
	fmt.Printf("Retrying (%d) elements\n", n)
	for k := 0; k < n; k++ {
		r := <-ret.queue
		err := ret.Process(r)
		if err != nil {
			fmt.Printf("retry failed with error: %s\n", err)
			ret.queue <- r
			continue
		}
		fmt.Println("retry success")
		continue

	}

}

func (ret *Retrier) Process(r *http.Request) error {

	var client = &http.Client{
		Timeout: time.Second * 2,
	}

	//fmt.Println(r)
	// Use the default HTTP client to send the request to the target server
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	//fmt.Println(resp)

	statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("service responded with %s", http.StatusText(resp.StatusCode))
	}
	return nil
}

func RandBool() bool {
	return rand.Intn(2) == 1
}
