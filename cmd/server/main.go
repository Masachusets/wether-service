package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Masachusets/wether-service/internal/http/geocoding"
	"github.com/Masachusets/wether-service/internal/http/meteo"
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

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	geoClient := geocoding.NewClient(httpClient)

	meteoClient := meteo.NewClient(httpClient)

	r.Get("/{city}", func(w http.ResponseWriter, r *http.Request) {
		city := chi.URLParam(r, "city")

		coords, err := geoClient.GetCoordinates(city)
		if err != nil {
			log.Println(err)
			return
		}

		res, err := meteoClient.GetTemperature(coords.Latitude, coords.Longitude)
		if err != nil {
			log.Println(err)
			return
		}
		
		raw, err := json.Marshal(res)
		if err != nil {
			log.Println(err)
			return
		}

		_, err = w.Write(raw)
		if err != nil {
			log.Println(err)
			return
		}
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
