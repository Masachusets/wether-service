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
	city     = "Minsk"
)

type Reading struct {
	Name        string
	Timestamp   time.Time
	Temperature float64
}

type Storage struct {
	data map[string][]Reading
	mu sync.RWMutex
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	storage := &Storage{
		data: make(map[string][]Reading),
	}

	r.Get("/{city}", func(w http.ResponseWriter, r *http.Request) {
		cityName := chi.URLParam(r, "city")

		storage.mu.RLock()
		defer storage.mu.RUnlock()

		// var reading Reading

		reading, ok := storage.data[cityName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
			return
		}

		raw, err := json.Marshal(reading)
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

	j, err := newCronJob(s, storage)
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		fmt.Println("starting server on port", httpPort)
		err = http.ListenAndServe(httpPort, r)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		defer wg.Done()

		fmt.Printf("starting job: %v\n", j.ID())
		s.Start()
	}()

	wg.Wait()
}

func newCronJob(s gocron.Scheduler, storage *Storage) (gocron.Job, error) {
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	geoClient := geocoding.NewClient(httpClient)

	meteoClient := meteo.NewClient(httpClient)

	// add a job to the scheduler
	job, err := s.NewJob(
		gocron.DurationJob(
			10*time.Second,
		),
		gocron.NewTask(
			func() {
				geoRes, err := geoClient.GetCoordinates(city)
				if err != nil {
					log.Println(err)
					return
				}

				meteoRes, err := meteoClient.GetTemperature(geoRes.Latitude, geoRes.Longitude)
				if err != nil {
					log.Println(err)
					return
				}

				storage.mu.Lock()
				defer storage.mu.Unlock()

				timestamp, err := time.Parse("2006-01-02T15:04", meteoRes.Current.Time)
				if err != nil {
					log.Panicln(err)
					return 
				}

				storage.data[city] = append(
					storage.data[city],
					Reading{
						Name: city,
						Timestamp: timestamp,
						Temperature: meteoRes.Current.Temperature2m,
					},
				)

				fmt.Printf("updated data for city %s\n", city)
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return job, nil
}
