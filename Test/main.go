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
	ClientId string `json:"client-id"`
	Label    string `json:"label"`

	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`

	UltrasonicCM float64 `json:"ultrasonic_cm"`
	TofCM        float64 `json:"tof_cm"`

	RSSI int    `json:"rssi"`
	Net  string `json:"net"`
}

func main() {
	topic := "421342415"
	sonde := []Sonda{
		{
			ClientId: "id_TN_VL_0",
			Label:    "Rio Molini 0",
			Lat:      45.915771,
			Lon:      11.024346,
			Topic:    topic + "/ID_1_Vigili_VillaLagarina_TN/Rio_Molini_0",
		},
		{
			ClientId: "id_TN_VL_1",
			Label:    "Rio Val Morela 0",
			Lat:      45.921549,
			Lon:      11.032031,
			Topic:    topic + "/ID_1_Vigili_VillaLagarina_TN/Rio_Val_Morela_0",
		},
		{
			ClientId: "id_TN_VL_2",
			Label:    "Rio Piazzo 0",
			Lat:      45.924787,
			Lon:      11.034732,
			Topic:    topic + "/ID_1_Vigili_VillaLagarina_TN/Rio_Piazzo_0",
		},
		{
			ClientId: "id_TN_VL_3",
			Label:    "Rio San Clemente 0",
			Lat:      45.924960,
			Lon:      11.037627,
			Topic:    topic + "/ID_1_Vigili_VillaLagarina_TN/Rio_San_Clemente_0",
		},
	}

	opts := mqtt.NewClientOptions()

	opts.AddBroker("tcp://broker.hivemq.com:1883")
	opts.SetClientID("test_generator")

	client := mqtt.NewClient(opts)

	token := client.Connect()
	token.Wait()

	if token.Error() != nil {
		panic(token.Error())
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	networks := []string{
		"LTE",
		"4G",
		"3G",
		"EDGE",
	}

	for {
		for _, sonda := range sonde {
			ultrasonic_cm := math.Round((rng.Float64()*200+20)*100) / 100

			tof_offset := rng.Float64()*2 - 1

			tof_cm := math.Round((ultrasonic_cm+tof_offset)*100) / 100

			payload := Payload{
				ClientId: sonda.ClientId,
				Label:    sonda.Label,

				Lat: sonda.Lat,
				Lon: sonda.Lon,

				UltrasonicCM: ultrasonic_cm,
				TofCM:        tof_cm,

				RSSI: rng.Intn(32),
				Net:  networks[rng.Intn(len(networks))],
			}

			data, err := json.Marshal(payload)

			if err != nil {
				fmt.Println(err)
				continue
			}

			token := client.Publish(sonda.Topic, 0, false, data)

			token.Wait()

			fmt.Println(string(data))

			time.Sleep(time.Second)
		}
	}
}
