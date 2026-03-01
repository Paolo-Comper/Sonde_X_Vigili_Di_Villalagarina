package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"strings"
	"sort"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type DeviceState struct {
	ID        string  `json:"id"`       // client-id
	Topic     string  `json:"topic"`    // fiume1
	Label     string  `json:"label"`    // Adige
	Value     float64 `json:"value"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type Response struct {
	Data []DeviceState `json:"data"`
}

type Incoming struct {
	ClientID string  `json:"client-id"`
	Label    string  `json:"label"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Value    float64 `json:"value"`
}

var (
	state = make(map[string]DeviceState)
	mutex sync.Mutex
)

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://test.mosquitto.org:1883")
	opts.SetClientID("state_server")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	client.Subscribe("Sonde_X_Vigili_Di_Villalagarina/+", 0, on_message)

	http.HandleFunc("/state.json", state_handler)
	fmt.Println("Server HTTP su http://localhost:8080/state.json")
	http.ListenAndServe(":8080", nil)
}

func on_message(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()

	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return
	}

	device_topic := parts[1]

	var incoming Incoming
	err := json.Unmarshal(msg.Payload(), &incoming)
	if err != nil {
		fmt.Println("Errore parsing JSON:", err)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	state[device_topic] = DeviceState{
		ID:        incoming.ClientID,
		Topic:     device_topic,
		Label:     incoming.Label,
		Value:     incoming.Value,
		Latitude:  incoming.Lat,
		Longitude: incoming.Lon,
	}
}

func state_handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	mutex.Lock()
	defer mutex.Unlock()

	var resp Response
	for _, dev := range state {
		resp.Data = append(resp.Data, dev)
	}

	sort.Slice(resp.Data, func(i, j int) bool {
		return resp.Data[i].ID < resp.Data[j].ID
	})

	json.NewEncoder(w).Encode(resp)
}

