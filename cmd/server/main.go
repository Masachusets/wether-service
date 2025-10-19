package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-co-op/gocron/v2"
)

const (
	httpPort = ":3000"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})

	// create a scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		panic(err)
	}

	j, err := newCronJob(s)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func () {
		defer wg.Done()

		fmt.Println("starting server on port", httpPort)
		err = http.ListenAndServe(httpPort, r)
		if err != nil {
			panic(err)
		}
	}()

	go func () {
		defer wg.Done()

		fmt.Printf("starting job: %v\n", j.ID())
		s.Start()
	}()

	wg.Wait()
}

func newCronJob(s gocron.Scheduler) (gocron.Job, error) {
	// add a job to the scheduler
	job, err := s.NewJob(
		gocron.DurationJob(
			10*time.Second,
		),
		gocron.NewTask(
			func() {
				fmt.Println("Hello!")
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return job, nil
}
