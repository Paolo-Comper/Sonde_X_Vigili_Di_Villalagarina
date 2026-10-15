// Package sim pubblica dati di prova per piu' dispositivi, utili per provare
// la dashboard senza hardware (modalita' --test).
package sim

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Options configura la simulazione.
type Options struct {
	Broker   string        // host del broker MQTT
	Root     string        // primo livello del topic
	Interval time.Duration // intervallo tra un giro e l'altro
	Count    int           // numero di giri (0 = infinito)
}

// Run si connette al broker e pubblica finche' non termina o viene interrotta.
func Run(opts Options, logf func(format string, args ...any)) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	if opts.Root == "" {
		opts.Root = "421342415"
	}

	clientOpts := mqtt.NewClientOptions().
		AddBroker("tcp://" + opts.Broker).
		SetClientID(fmt.Sprintf("sonde-test-%08x", rand.Uint32())).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectTimeout(10 * time.Second)

	client := mqtt.NewClient(clientOpts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}
	defer client.Disconnect(250)

	devices := defaultDevices()
	for round := 1; opts.Count <= 0 || round <= opts.Count; round++ {
		for i := range devices {
			topic, payload := devices[i].message(opts.Root)
			pub := client.Publish(topic, 0, false, payload)
			pub.WaitTimeout(5 * time.Second)
			if err := pub.Error(); err != nil {
				logf("publish %s: %v", topic, err)
				continue
			}
			logf("%s -> %s", topic, payload)
		}
		if opts.Count > 0 && round >= opts.Count {
			break
		}
		time.Sleep(opts.Interval)
	}
	return nil
}

type device struct {
	id    string
	label string
	kind  string // "level" | "air"
	lat   float64
	lon   float64

	level float64
	pm25  float64
	temp  float64
	hum   float64
	press float64
}

func defaultDevices() []device {
	return []device{
		{id: "Rio_Molini_0", label: "Rio Molini 0", kind: "level", lat: 45.915771, lon: 11.024346, level: 120},
		{id: "Rio_Val_Morela_0", label: "Rio Val Morela 0", kind: "level", lat: 45.921549, lon: 11.032031, level: 145},
		{id: "Rio_Piazzo_0", label: "Rio Piazzo 0", kind: "level", lat: 45.924787, lon: 11.034732, level: 90},
		{id: "Rio_San_Clemente_0", label: "Rio San Clemente 0", kind: "level", lat: 45.924960, lon: 11.037627, level: 105},
		{id: "rpi-zero-1", label: "Aria Rovereto Centro", kind: "air", lat: 45.890000, lon: 11.040000, pm25: 12, temp: 15, hum: 65, press: 1010},
		{id: "rpi-zero-2", label: "Aria Villalagarina", kind: "air", lat: 45.913500, lon: 11.030100, pm25: 9, temp: 14, hum: 70, press: 1009},
	}
}

func (d *device) message(root string) (string, []byte) {
	topic := root + "/" + d.id
	now := float64(time.Now().UnixMilli()) / 1000.0

	var payload any
	if d.kind == "level" {
		d.level = walk(d.level, 6, 20, 320)
		us := round2(d.level + noise(1.5))
		tof := round2(us + noise(0.8))
		payload = map[string]any{
			"client-id":     "id_" + d.id,
			"label":         d.label,
			"lat":           d.lat,
			"lon":           d.lon,
			"ultrasonic_cm": us,
			"tof_cm":        tof,
			"rssi":          10 + rand.Intn(22),
			"net":           []string{"LTE", "4G", "3G"}[rand.Intn(3)],
		}
	} else {
		d.pm25 = walk(d.pm25, 4, 0, 90)
		d.temp = walk(d.temp, 0.6, -5, 35)
		d.hum = walk(d.hum, 2, 20, 100)
		d.press = walk(d.press, 1.2, 980, 1040)
		pm1 := round2(math.Max(0, d.pm25-4+noise(1)))
		pm10 := round2(d.pm25 + 5 + noise(2))
		tempDHT := round2(d.temp + noise(0.4))
		humDHT := round2(clamp(d.hum+noise(2), 0, 100))
		tempBME := round2(d.temp + noise(0.3))
		humBME := round2(clamp(d.hum+noise(1), 0, 100))
		pressBME := round2(d.press*100 + noise(30))
		payload = map[string]any{
			"timestamp": now,
			"label":     d.label,
			"pms": map[string]any{
				"pm1": pm1, "pm25": round2(d.pm25), "pm10": pm10,
				"temperature": round2(d.temp + noise(0.5)), "humidity": round2(clamp(d.hum+noise(3), 0, 100)),
			},
			"dht22":   map[string]any{"temperature": tempDHT, "humidity": humDHT},
			"bme280":  map[string]any{"temperature": tempBME, "humidity": humBME, "pressure": pressBME},
			"virtual": map[string]any{"temperature": round2((tempDHT + tempBME) / 2), "humidity": round2((humDHT + humBME) / 2), "pressure": pressBME},
			"gps":     map[string]any{"lat": d.lat, "lon": d.lon, "alt": 180.0, "sats": 6 + rand.Intn(6), "valid": true},
		}
	}

	data, _ := json.Marshal(payload)
	return topic, data
}

func walk(v, step, min, max float64) float64 {
	return clamp(v+noise(step), min, max)
}

func noise(amplitude float64) float64 {
	return (rand.Float64()*2 - 1) * amplitude
}

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
