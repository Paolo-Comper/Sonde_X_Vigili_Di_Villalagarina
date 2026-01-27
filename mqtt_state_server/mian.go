package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type DeviceState struct {
	ID        string  `json:"id"`
	Value     float64 `json:"value"`
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type Response struct {
	Data []DeviceState `json:"data"`
}

var (
	state = make(map[string]DeviceState)
	mutex sync.Mutex
)

func main() {
	// --- MQTT ---
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://test.mosquitto.org:1883")
	opts.SetClientID("state_server")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	client.Subscribe("Sonde_X_Vigili_Di_Villalagarina/+/+", 0, on_message)

	// --- HTTP ---
	http.HandleFunc("/state.json", state_handler)
	fmt.Println("Server HTTP su http://localhost:6969/state.json")
	http.ListenAndServe(":6969", nil)
}

func on_message(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// topic: Sonde_X_Vigili_Di_Villalagarina/esp1/value
	var device, field string
	fmt.Sscanf(topic, "Sonde_X_Vigili_Di_Villalagarina/%s/%s", &device, &field)

	mutex.Lock()
	defer mutex.Unlock()

	s := state[device]
	s.ID = device

	switch field {
	case "value":
		fmt.Sscanf(payload, "%f", &s.Value)
	case "lat":
		fmt.Sscanf(payload, "%f", &s.Latitude)
	case "lon":
		fmt.Sscanf(payload, "%f", &s.Longitude)
	}

	state[device] = s
}

func state_handler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()

	var resp Response

	for _, dev := range state {
		resp.Data = append(resp.Data, dev)
	}

	json.NewEncoder(w).Encode(resp)
}
