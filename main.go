package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type NlwknResponse struct {
	Stations []Station `json:"getStammdatenResult"`
}

type Station struct {
	Name      string       `json:"name"`
	Operator  string       `json:"betreiber"`
	Longitude string       `json:"Longitude"`
	Latitude  string       `json:"Latitude"`
	Parameter []DataParams `json:"Parameter"`
	Storage   bool         `json:"IstSpeicher"` // talsperre, rückhaltebecken
}

type DataParams struct {
	Traces []Trace `json:"datenspuren"`
	Name   string  `json:"name"`
	Unit   string  `json:"einheit"`
}

type Trace struct {
	AlertLevel    float64 `json:"AktuelleMeldeStufe"`
	Level         float64 `json:"AktuellerMesswert"`
	Text          string  `json:"AktuellerMesswert_Text"`
	Timestamp     string  `json:"AktuellerMesswert_Zeitpunkt"`
	Color         string  `json:"Farbe"`
	ParameterName string  `json:"ParameterName"`
	ParameterUnit string  `json:"ParameterEinheit"`
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

var data = GeoJSONFeatureCollection{}
var lastTimeFetched time.Time

func main() {
	app := fiber.New()
	fetchNlwknData()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
	}))

	app.Get("/pegelstaende.geojson", func(c *fiber.Ctx) error {
		// fetch new data every 5 minutes
		if time.Since(lastTimeFetched).Minutes() > 5 {
			fetchNlwknData()
			lastTimeFetched = time.Now()
		}

		return c.JSON(data)
	})

	app.Get("/xyz", func(c *fiber.Ctx) error {
		d := fetchNlwknData()
		return c.JSON(d)
	})

	log.Fatal(app.Listen(":3000"))
}

func fetchNlwknData() NlwknResponse {
	// make fetch request to pegelonline
	resp, err := http.Get("https://bis.azure-api.net/PegelonlinePublic/REST/stammdaten/stationen/All?key=9dc05f4e3b4a43a9988d747825b39f43")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// parse json response
	var responeData NlwknResponse
	err = json.NewDecoder(resp.Body).Decode(&responeData)
	if err != nil {
		log.Fatal(err)
	}

	data = ResponseToGeoJson(responeData)
	return responeData
}

func ResponseToGeoJson(response NlwknResponse) GeoJSONFeatureCollection {
	geoJson := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: []GeoJSONFeature{},
	}

	for _, station := range response.Stations {
		longitude, _ := strconv.ParseFloat(station.Longitude, 64)
		latitude, _ := strconv.ParseFloat(station.Latitude, 64)

		// check if station has pegelstand
		var unit string
		var measurement string
		var color string

		if len(station.Parameter) == 0 || len(station.Parameter[0].Traces) == 0 {
			measurement = "-"
			unit = "-"
		} else {
			measurement = station.Parameter[0].Traces[0].Text
			color = station.Parameter[0].Traces[0].Color
			unit = station.Parameter[0].Traces[0].ParameterUnit
		}

		feature := GeoJSONFeature{
			Type: "Feature",
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{latitude, longitude},
			},
			Properties: map[string]interface{}{
				"name":              station.Name,
				"betreiber":         station.Operator,
				"marker-color":      color,
				"AktuellerMesswert": measurement,
				"Einheit":           unit,
				"IstSpeicher":       station.Storage,
			},
		}

		geoJson.Features = append(geoJson.Features, feature)
	}

	return geoJson
}
