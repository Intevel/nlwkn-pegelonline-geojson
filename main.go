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
	Name        string       `json:"name"`
	Betreiber   string       `json:"betreiber"`
	Longitude   string       `json:"Longitude"`
	Latitude    string       `json:"Latitude"`
	Parameter   []DataParams `json:"Parameter"`
	IstSpeicher bool         `json:"IstSpeicher"` // talsperre, rückhaltebecken
}

type DataParams struct {
	Datenspuren []Datenspur `json:"datenspuren"`
	Name        string      `json:"name"`
	Einheit     string      `json:"einheit"`
}

type Datenspur struct {
	AktuelleMeldeStufe          float64 `json:"AktuelleMeldeStufe"`
	AktuellerMesswert           float64 `json:"AktuellerMesswert"`
	AktuellerMesswert_Text      string  `json:"AktuellerMesswert_Text"`
	AktuellerMesswert_Zeitpunkt string  `json:"AktuellerMesswert_Zeitpunkt"`
	Farbe                       string  `json:"Farbe"`
	ParameterName               string  `json:"ParameterName"`
	ParameterEinheit            string  `json:"ParameterEinheit"`
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
		var Einheit string
		var AktuellerMesswert string
		var Farbe string

		if len(station.Parameter) == 0 || len(station.Parameter[0].Datenspuren) == 0 {
			AktuellerMesswert = "-"
			Einheit = "-"
		} else {
			AktuellerMesswert = station.Parameter[0].Datenspuren[0].AktuellerMesswert_Text
			Farbe = station.Parameter[0].Datenspuren[0].Farbe
			Einheit = station.Parameter[0].Datenspuren[0].ParameterEinheit
		}

		feature := GeoJSONFeature{
			Type: "Feature",
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{latitude, longitude},
			},
			Properties: map[string]interface{}{
				"name":              station.Name,
				"betreiber":         station.Betreiber,
				"marker-color":      Farbe,
				"AktuellerMesswert": AktuellerMesswert,
				"Einheit":           Einheit,
				"IstSpeicher":       station.IstSpeicher,
			},
		}

		geoJson.Features = append(geoJson.Features, feature)
	}

	return geoJson
}
