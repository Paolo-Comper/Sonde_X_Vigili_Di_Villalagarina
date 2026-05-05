package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/glebarez/go-sqlite"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type DeviceState struct {
	ID        string  `json:"id"`
	Topic     string  `json:"topic"`
	Label     string  `json:"label"`
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

type Config struct {
	mqttBroker string
	clientID   string
	topic      string

	httpPort string

	snapshotInterval time.Duration
	snapshotDir      string
	databasePath     string
}

var (
	cfg = Config{
		mqttBroker:       "tcp://broker.hivemq.com:1883",
		clientID:         "state_server",
		topic:            "Sonde_X_Vigili_Di_Villalagarina/+",
		httpPort:         ":8080",
		snapshotInterval: 5 * time.Second,
		snapshotDir:      "snapshots",
		databasePath:     "./database.db",
	}

	state = make(map[string]DeviceState)
	mutex sync.RWMutex

	db *sql.DB
)

func main() {
	init_db()
	defer db.Close()

	go snapshot_loop()
	go start_http_server()

	start_mqtt()

	select {} // blocca per sempre
}

func start_mqtt() {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.mqttBroker).
		SetClientID(cfg.clientID).
		SetAutoReconnect(true)

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	client.Subscribe(cfg.topic, 0, on_message)
}

func start_http_server() {
	http.HandleFunc("/state.json", state_handler)

	fmt.Println("HTTP server on http://localhost" + cfg.httpPort + "/state.json")
	http.ListenAndServe(cfg.httpPort, nil)
}

func on_message(client mqtt.Client, msg mqtt.Message) {
	device_topic := parse_topic(msg.Topic())
	if device_topic == "" {
		return
	}

	var incoming Incoming
	if err := json.Unmarshal(msg.Payload(), &incoming); err != nil {
		fmt.Println("JSON parse error:", err)
		return
	}

	dev := DeviceState{
		ID:        incoming.ClientID,
		Topic:     device_topic,
		Label:     incoming.Label,
		Value:     incoming.Value,
		Latitude:  incoming.Lat,
		Longitude: incoming.Lon,
	}

	update_state(device_topic, dev)
	insert_measurement(dev)
}

func parse_topic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func update_state(key string, dev DeviceState) {
	mutex.Lock()
	state[key] = dev
	mutex.Unlock()
}

func snapshot_loop() {
	ticker := time.NewTicker(cfg.snapshotInterval)
	defer ticker.Stop()

	for range ticker.C {
		mutex.RLock()

		for _, dev := range state {
			if err := insert_measurement(dev); err != nil {
				fmt.Println("snapshot insert error:", err)
			}
		}

		mutex.RUnlock()
	}
}

func state_handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	mutex.RLock()

	resp := Response{
		Data: make([]DeviceState, 0, len(state)),
	}

	for _, dev := range state {
		resp.Data = append(resp.Data, dev)
	}

	mutex.RUnlock()

	sort.Slice(resp.Data, func(i, j int) bool {
		return resp.Data[i].ID < resp.Data[j].ID
	})

	json.NewEncoder(w).Encode(resp)
}

func init_db() {
	var err error

	db, err = sql.Open("sqlite", cfg.databasePath)
	if err != nil {
		panic(err)
	}

	if _, err := create_table(); err != nil {
		panic(err)
	}

	fmt.Println("SQLite connected")
}

func create_table() (sql.Result, error) {
	query := `
	CREATE TABLE IF NOT EXISTS measurements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		topic TEXT NOT NULL,
		label TEXT NOT NULL,
		value REAL NOT NULL,
		lat REAL NOT NULL,
		lon REAL NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	return db.Exec(query)
}

func insert_measurement(dev DeviceState) error {
	query := `
	INSERT INTO measurements (
		device_id, topic, label, value, lat, lon
	) VALUES (?, ?, ?, ?, ?, ?);`

	_, err := db.Exec(
		query,
		dev.ID,
		dev.Topic,
		dev.Label,
		dev.Value,
		dev.Latitude,
		dev.Longitude,
	)

	return err
}
