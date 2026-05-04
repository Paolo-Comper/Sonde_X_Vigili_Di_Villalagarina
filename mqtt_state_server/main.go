package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type DeviceState struct {
	ID        string  `json:"id"`    // client-id
	Topic     string  `json:"topic"` // fiume1
	Label     string  `json:"label"` // Adige
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
	opts.AddBroker("tcp://broker.hivemq.com:1883")
	opts.SetClientID("state_server")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	client.Subscribe("Sonde_X_Vigili_Di_Villalagarina/+", 0, on_message)

	http.HandleFunc("/state.json", state_handler)
	fmt.Println("Server HTTP su http://localhost:8080/state.json")

	go snapshot_loop()

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

func snapshot_loop() {
	ticker := time.NewTicker(5 * time.Minute)

	defer ticker.Stop()

	for range ticker.C {
		save_snapshot()
	}
}

func save_snapshot() {
	mutex.Lock()

	var resp Response
	for _, dev := range state {
		resp.Data = append(resp.Data, dev)
	}

	mutex.Unlock()

	// nome file con timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("snapshot_data_%s.json", timestamp)

	// cartella (creata se non esiste)
	dir := "snapshots"
	os.MkdirAll(dir, os.ModePerm)

	full_path := filepath.Join(dir, filename)

	file, err := os.Create(full_path)
	if err != nil {
		fmt.Println("Errore creazione file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(resp)
	if err != nil {
		fmt.Println("Errore scrittura JSON:", err)
		return
	}

	fmt.Println("Snapshot salvato:", full_path)
}
