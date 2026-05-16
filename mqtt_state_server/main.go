package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/glebarez/go-sqlite"
	"net/http"
	"os"
	"path/filepath"
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

	dbPath string

	dbInterval       time.Duration
	snapshotInterval time.Duration
	snapshotDir      string
}

var cfg = Config{
	mqttBroker: "tcp://broker.hivemq.com:1883",
	clientID:   "state_server",
	topic:      "Sonde_X_Vigili_Di_Villalagarina/+",
	httpPort:   ":8080",

	dbPath: "./database.db",

	dbInterval:       5 * time.Second,
	snapshotInterval: 10 * time.Minute,
	snapshotDir:      "snapshots",
}

var (
	state = make(map[string]DeviceState)
	mutex sync.RWMutex

	db *sql.DB
)

func main() {
	init_db()
	defer db.Close()

	go mqtt_loop()
	go http_server()

	go db_loop()
	go snapshot_loop()

	select {}
}

// MQTT

func mqtt_loop() {
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

func on_message(client mqtt.Client, msg mqtt.Message) {
	device_topic := parse_topic(msg.Topic())
	if device_topic == "" {
		return
	}

	var incoming Incoming
	if err := json.Unmarshal(msg.Payload(), &incoming); err != nil {
		fmt.Println("JSON error:", err)
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

	mutex.Lock()
	state[device_topic] = dev
	mutex.Unlock()
}

func parse_topic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// HTTP

func http_server() {
	http.HandleFunc("/state.json", state_handler)

	fmt.Println("HTTP on http://localhost" + cfg.httpPort + "/state.json")
	http.ListenAndServe(cfg.httpPort, nil)
}

func state_handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	mutex.RLock()

	resp := Response{
		Data: make([]DeviceState, 0, len(state)),
	}

	for _, v := range state {
		resp.Data = append(resp.Data, v)
	}

	mutex.RUnlock()

	sort.Slice(resp.Data, func(i, j int) bool {
		return strings.ToLower(resp.Data[i].Label) <
			strings.ToLower(resp.Data[j].Label)
	})

	json.NewEncoder(w).Encode(resp)
}

// DATABASE LOOP (5s)

func db_loop() {
	ticker := time.NewTicker(cfg.dbInterval)
	defer ticker.Stop()

	for range ticker.C {
		mutex.RLock()

		for _, dev := range state {
			if err := insert_measurement(dev); err != nil {
				fmt.Println("DB insert error:", err)
			}
		}

		mutex.RUnlock()
	}
}

func insert_measurement(dev DeviceState) error {
	query := `
	INSERT INTO measurements (
		device_id, topic, label, value, lat, lon
	) VALUES (?, ?, ?, ?, ?, ?);`

	_, err := db.Exec(query,
		dev.ID,
		dev.Topic,
		dev.Label,
		dev.Value,
		dev.Latitude,
		dev.Longitude,
	)

	return err
}

// SNAPSHOT LOOP (10 min)

func snapshot_loop() {
	ticker := time.NewTicker(cfg.snapshotInterval)
	defer ticker.Stop()

	for range ticker.C {
		save_snapshot()
	}
}

func save_snapshot() {
	mutex.RLock()

	resp := Response{
		Data: make([]DeviceState, 0, len(state)),
	}

	for _, v := range state {
		resp.Data = append(resp.Data, v)
	}

	mutex.RUnlock()

	os.MkdirAll(cfg.snapshotDir, os.ModePerm)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("snapshot_%s.json", timestamp)
	full_path := filepath.Join(cfg.snapshotDir, filename)

	file, err := os.Create(full_path)
	if err != nil {
		fmt.Println("snapshot error:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)

	fmt.Println("Snapshot saved:", full_path)
}

// DB INIT

func init_db() {
	var err error

	db, err = sql.Open("sqlite", cfg.dbPath)
	if err != nil {
		panic(err)
	}

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

	if _, err := db.Exec(query); err != nil {
		panic(err)
	}

	fmt.Println("SQLite ready")
}
