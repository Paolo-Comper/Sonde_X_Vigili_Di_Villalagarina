package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Sonda struct {
	ClientId string
	Label    string
	Lat      float64
	Lon      float64
	Topic    string
}

type Payload struct {
	ClientId string  `json:"client-id"`
	Label    string  `json:"label"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Value    float64 `json:"value"`
}

func main() {
	sonde := []Sonda{
		{
			ClientId: "esp32_1",
			Label:    "Rio Molini",
			Lat:      45.891,
			Lon:      11.041,
			Topic:    "Sonde_X_Vigili_Di_Villalagarina/rio_molini",
		},
		{
			ClientId: "esp32_2",
			Label:    "Rio Val Morela",
			Lat:      45.892,
			Lon:      11.042,
			Topic:    "Sonde_X_Vigili_Di_Villalagarina/rio_val_morela",
		},
		{
			ClientId: "esp32_2",
			Label:    "Rio Piazzo",
			Lat:      45.892,
			Lon:      11.042,
			Topic:    "Sonde_X_Vigili_Di_Villalagarina/rio_piazzo",
		},
		{
			ClientId: "esp32_2",
			Label:    "San Clemente",
			Lat:      45.892,
			Lon:      11.042,
			Topic:    "Sonde_X_Vigili_Di_Villalagarina/san_clemente",
		},
		{
			ClientId: "esp32_2",
			Label:    "Adige",
			Lat:      45.892,
			Lon:      11.042,
			Topic:    "Sonde_X_Vigili_Di_Villalagarina/adige",
		},
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://broker.hivemq.com:1883")

	client := mqtt.NewClient(opts)

	token := client.Connect()
	token.Wait()

	if token.Error() != nil {
		panic(token.Error())
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		for _, sonda := range sonde {
			payload := Payload{
				ClientId: sonda.ClientId,
				Label:    sonda.Label,
				Lat:      sonda.Lat,
				Lon:      sonda.Lon,
				Value:    math.Round(rng.Float64()*10*1000) / 1000,
			}

			data, err := json.Marshal(payload)

			if err != nil {
				fmt.Println(err)
				continue
			}

			token := client.Publish(
				sonda.Topic,
				0,
				false,
				data,
			)

			token.Wait()

			fmt.Println(string(data))

			time.Sleep(time.Second)
		}
	}
}
