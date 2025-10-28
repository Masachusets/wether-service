package main

import (
	"context"
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
	"github.com/jackc/pgx/v5"
)

const (
	httpPort = ":3000"
	city     = "Minsk"
	db_conn  = "postgresql://wether:1234@localhost:5432/wether_db"
)

type Reading struct {
	Name        string    `db:"name"`
	Timestamp   time.Time `db:"timestamp"`
	Temperature float64   `db:"temperature"`
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, db_conn)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		panic(err)
	}
	defer conn.Close(ctx)

	r.Get("/{city}", func(w http.ResponseWriter, r *http.Request) {
		cityName := chi.URLParam(r, "city")

		var reading Reading

		query := "select name, timestamp, temperature from reading where name = $1 order by timestamp desc limit 1"

		err := conn.QueryRow(
			ctx,
			query,
			cityName,
		).Scan(
			&reading.Name,
			&reading.Timestamp,
			&reading.Temperature,
		)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(err.Error()))
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

	j, err := newCronJob(s, ctx, conn)
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

func newCronJob(s gocron.Scheduler, ctx context.Context, conn *pgx.Conn) (gocron.Job, error) {
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

				timestamp, err := time.Parse("2006-01-02T15:04", meteoRes.Current.Time)
				if err != nil {
					log.Panicln(err)
					return
				}

				query := "insert into reading (name, timestamp, temperature) values ($1, $2, $3)"
				_, err = conn.Exec(
					ctx, 
					query, 
					city, 
					timestamp, 
					meteoRes.Current.Temperature2m,
				)
				if err != nil {
					log.Println(err)
					return
				}

				fmt.Printf(
					"%v updated data for city %s\n", 
					timestamp.Format("2006-01-02 15:04:05"), 
					city,
				)
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return job, nil
}
