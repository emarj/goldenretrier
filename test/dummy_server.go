package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	// Set up a handler for all incoming requests
	http.HandleFunc("/", randomResponseHandler)

	// Start the web server on port 8080
	url := "localhost:8081"
	log.Printf("Starting server on :%s...\n", url)
	if err := http.ListenAndServe(url, nil); err != nil {
		log.Println("Error starting server:", err)
	}
}

func randomResponseHandler(w http.ResponseWriter, r *http.Request) {
	// Generate a random number (0 or 1)
	randomNumber := rand.Intn(3)

	time.Sleep(100 * time.Millisecond)

	switch randomNumber {
	case 0:
		delay := 10
		log.Printf("Will respond in %d seconds\n", delay)
		time.Sleep(time.Duration(delay) * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success!"))
	case 1:
		// Respond with a successful status code (200)
		log.Println("Success")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success!"))
	case 2:
		// Respond with an error status code (e.g., 500)
		log.Println("Error")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
}
