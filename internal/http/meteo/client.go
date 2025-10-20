package meteo

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Response struct {
	Current struct {
		Time          string  `json:"time"`
		Temperature2m float64 `json:"temperature_2m"`	
	}
}

type meteoClient struct {
	httpClient *http.Client
}

func NewMeteoClient(httpClient *http.Client) *meteoClient {
	return &meteoClient{
		httpClient: httpClient,
	}
}

func (m *meteoClient) GetTemperature(lat, long float64) (Response, error) {
	res, err := m.httpClient.Get(
		fmt.Sprintf(
			"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m",
			lat,
			long,
		),
	)
	if err != nil {
		return Response{}, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("status code %d:", res.StatusCode)
	}

	var response Response
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return Response{}, err
	}

	return response, nil
}
