package main

import (
	"fmt"
	"log"
	"time"
)

type Item[T any] struct {
	I          T
	RetryCount uint
	Time       time.Time
}

type Action[T any] func(Item[T]) error

type Retrier[T any] struct {
	items        chan Item[T]
	capacity     int
	queue        chan Item[T]
	timeout      time.Duration
	action       Action[T]
	abortOnError bool
	maxRetries   uint
	maxAge       time.Duration
}

func NewRetrier[T any](timeout time.Duration, capacity int, abortOnError bool, action Action[T]) *Retrier[T] {
	return &Retrier[T]{
		capacity:     capacity,
		queue:        make(chan Item[T], capacity),
		items:        make(chan Item[T], capacity),
		timeout:      timeout,
		action:       action,
		abortOnError: abortOnError,
	}
}

func (ret *Retrier[T]) Add(i T) error {
	item := Item[T]{
		I:          i,
		RetryCount: 0,
		Time:       time.Now(),
	}

	//log.Print("adding item")
	select {
	case ret.queue <- item:
		log.Println("item successfully added to queue")
	default:
		log.Println("queue is full")
		return fmt.Errorf("the queue is full")
	}

	return nil
}

func (ret *Retrier[T]) Start() error {
	ticker := time.NewTicker(ret.timeout)
	go func() {
		for {
			<-ticker.C
			ret.Retry()
		}
	}()

	return nil
}

func (ret *Retrier[T]) Retry() {

out:
	for len(ret.queue) > 0 {
		i := <-ret.queue
		select {
		case ret.items <- i:
			log.Println("item successfully added to active list")
		default:
			log.Println("full capacity")
			break out
		}
	}

	n := len(ret.items)
	if n == 0 {
		return
	}

	log.Printf("retrying (%d) elements\n", n)
	for k := 0; k < n; k++ {
		i := <-ret.items
		i.RetryCount += 1

		log.Printf("\tretry n. %s", nOfMaxStr(i.RetryCount, ret.maxRetries))

		if ret.maxAge > 0 && time.Since(i.Time) >= ret.maxAge {
			log.Printf("\tmax age reached (%s)\n", ret.maxAge)
			continue
		}

		err := ret.action(i)
		if err != nil {
			log.Printf("\tfailed with error: %s\n", err)

			if ret.maxRetries == 0 || i.RetryCount < ret.maxRetries {
				ret.items <- i
			} else {
				log.Printf("\tmax number of retries reached (%s)\n", nOfMaxStr(i.RetryCount, ret.maxRetries))
			}

			if ret.abortOnError {
				log.Println("aborting...")
				// Remove and reinsert remaining items in queue
				for k += 1; k < n; k++ {
					i = <-ret.items
					ret.items <- i
				}
				return
			}

			continue
		}
		// No errors, success!
		log.Printf("\tsuccess after %d retries / %s \n", i.RetryCount, time.Since(i.Time).Round(time.Second))
	}

	if len(ret.items) == 0 {
		log.Println("queue is empty")
	}

}

func nOfMaxStr(n, max uint) string {
	if max != 0 {
		return fmt.Sprintf("%d of %d", n, max)
	} else {
		return fmt.Sprintf("%d", n)
	}
}
