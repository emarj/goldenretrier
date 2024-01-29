package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var reqTimeout int
var Debug bool

func main() {

	var forwardURL string
	flag.StringVar(&forwardURL, "to", "http://localhost:8081", "")
	var timeout int
	flag.IntVar(&timeout, "timeout", 5000, "timeout between attempts in ms")
	var capacity int
	flag.IntVar(&capacity, "capacity", 100, "")
	// Global variable
	flag.IntVar(&reqTimeout, "requestTimeout", 3000, "timeout before canceling a request in ms")
	// Global Debug flag
	flag.BoolVar(&Debug, "debug", false, "enable to log requests. Specify as -debug=value")
	var abortOnError bool
	flag.BoolVar(&abortOnError, "abortOnError", true, "abort retry on the first encountered error. This assures that the order of requests is preserved. Specify as -abortOnError=value")
	var maxAgeStr string
	flag.StringVar(&maxAgeStr, "maxAge", "0", "max age")
	var maxRetries int
	flag.IntVar(&maxRetries, "maxRetries", 0, "number of max retries")

	flag.Parse()

	var maxAge time.Duration
	maxAge, err := time.ParseDuration(maxAgeStr)
	if err != nil {
		panic(fmt.Sprintf("maxAge: %s", err))
	}

	if maxRetries < 0 {
		panic("maxRetries should be a non-negative number")
	}

	fmt.Println("abortOnError", abortOnError)

	// Create a context to listen for signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to wait for active connections to finish
	var wg sync.WaitGroup

	ret := NewRetrier(
		time.Duration(timeout)*time.Millisecond,
		capacity,
		abortOnError,
		RetryRequest,
	)

	ret.maxRetries = uint(maxRetries)
	ret.maxAge = maxAge
	ret.Start()

	// Start the web server in a separate goroutine
	server := &http.Server{Addr: "localhost:8080", Handler: http.DefaultServeMux}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		proxyReq, err := cloneRequest(r, forwardURL)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = ret.Add(proxyReq)
		if err != nil {
			w.WriteHeader(http.StatusInsufficientStorage)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Printf("Starting retrier on %s, forwarding to %s\n", server.Addr, forwardURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting server: %v\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	// Set up a channel to listen for interrupt signals
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-interruptChan:
		log.Println("Received interrupt signal. Shutting down...")
		break
	case <-ctx.Done():
		// Context canceled, shut down gracefully
		break
	}

	// Create a context with a timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shut down the server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v\n", err)
	}

	// Wait for active connections to finish before exiting
	wg.Wait()
}

func cloneRequest(r *http.Request, forwardUrl string) (*http.Request, error) {
	var result *http.Request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return result, err
	}

	url := *r.URL // make a copy of request url
	if forwardUrl == "" {
		url.Scheme = "http"
		url.Host = r.Host // Same host as request
	} else {
		baseURL, err := url.Parse(forwardUrl)
		if err != nil {
			return nil, err
		}
		url.Scheme = baseURL.Scheme
		url.Host = baseURL.Host
	}
	urlStr := url.String()

	result, err = http.NewRequest(r.Method, urlStr, bytes.NewReader(body))
	if err != nil {
		return result, err
	}

	result.Header = r.Header

	return result, nil
}

func RetryRequest(r Item[*http.Request]) error {

	var client = &http.Client{
		Timeout: time.Duration(reqTimeout) * time.Millisecond,
	}

	// Clone the request to avoid issues in case of failure
	req, err := cloneRequest(r.I, "")
	if err != nil {
		return err
	}

	//log.Println(r)
	if Debug {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Println(err)
		}
		log.Printf("--- Request ---\n%s\n---\n", string(dump))
	}

	log.Println("\t", "making request")
	// Use the default HTTP client to send the request to the target server
	res, err := client.Do(req)
	if err != nil {
		urlErr := err.(*url.Error)
		return urlErr.Err
	}
	defer res.Body.Close()

	if Debug {
		/* dump, err := httputil.DumpResponse(res, false)
		if err != nil {
			log.Println(err)
		}
		log.Printf("--- Response ---\n%s\n---\n", string(dump)) */
		log.Printf("--- Response ---\n%s\n---\n", res.Status)
	}

	statusOK := res.StatusCode >= 200 && res.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("service responded with: %d %s", res.StatusCode, http.StatusText(res.StatusCode))
	}
	return nil
}
