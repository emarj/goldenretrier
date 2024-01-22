package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Create a context to listen for signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to wait for active connections to finish
	var wg sync.WaitGroup

	ret, _ := NewRetrier("http://192.168.1.1")
	ret.Start()

	/* ret.Add(&http.Request{Host: "dunno"})
	fmt.Println(ret.Add(&http.Request{Host: "dasdas"})) */

	// Start the web server in a separate goroutine
	server := &http.Server{Addr: "localhost:8080", Handler: http.DefaultServeMux}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println(r.URL.String())
		proxyReq, err := http.NewRequest(r.Method, "http://192.168.1.1"+r.RequestURI, bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// We may want to filter some headers, otherwise we could just use a shallow copy
		// proxyReq.Header = req.Header
		proxyReq.Header = make(http.Header)
		for h, val := range r.Header {
			proxyReq.Header[h] = val
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

		fmt.Println("Starting server on :8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	waitForInterruptSignal(ctx, server)

	// Wait for active connections to finish before exiting
	wg.Wait()
	fmt.Println("Server gracefully shut down.")
}

func waitForInterruptSignal(ctx context.Context, server *http.Server) {
	// Set up a channel to listen for interrupt signals
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-interruptChan:
		fmt.Println("Received interrupt signal. Shutting down...")
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
		fmt.Printf("Error during server shutdown: %v\n", err)
	}
}
