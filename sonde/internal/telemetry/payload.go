// Package telemetry definisce il payload unificato accettato da sonde e la
// normalizzazione verso un unico valore Reading.
//
// sonde riceve dati da due famiglie di dispositivi:
//
//   - livello: le sonde idrometriche (ultrasuoni AJ-SR04M + ToF VL53L5CX),
//     che pubblicano un JSON piatto.
//   - aria:    le stazioni MIMOSA, che pubblicano un JSON annidato
//     (pms / dht22 / bme280 / virtual / gps).
//
// Le due forme sono decodificate dalla stessa struct Payload; un dispositivo
// puo' inviare entrambe le famiglie nello stesso messaggio (Kind == KindMixed).
package telemetry

import (
	"encoding/json"
	"strings"
	"time"
)

// Kind classifica il contenuto di una Reading.
type Kind string

const (
	KindLevel   Kind = "livello"
	KindAir     Kind = "aria"
	KindMixed   Kind = "misto"
	KindUnknown Kind = "sconosciuto"
)

// PMS e' il ramo del sensore di particolato.
type PMS struct {
	PM1         *float64 `json:"pm1"`
	PM25        *float64 `json:"pm25"`
	PM10        *float64 `json:"pm10"`
	Temperature *float64 `json:"temperature"`
	Humidity    *float64 `json:"humidity"`
}

// DHT22 e' il ramo del sensore di temperatura/umidita'.
type DHT22 struct {
	Temperature *float64 `json:"temperature"`
	Humidity    *float64 `json:"humidity"`
}

// BME280 e' il ramo del sensore di temperatura/umidita'/pressione.
type BME280 struct {
	Temperature *float64 `json:"temperature"`
	Humidity    *float64 `json:"humidity"`
	Pressure    *float64 `json:"pressure"`
}

// Virtual e' il ramo calcolato/fuso di MIMOSA; ha la precedenza piu' alta.
type Virtual struct {
	Temperature *float64 `json:"temperature"`
	Humidity    *float64 `json:"humidity"`
	Pressure    *float64 `json:"pressure"`
}

// GPS e' il ramo di posizione usato dai payload MIMOSA.
type GPS struct {
	Lat   *float64 `json:"lat"`
	Lon   *float64 `json:"lon"`
	Alt   *float64 `json:"alt"`
	Sats  *int     `json:"sats"`
	Valid *bool    `json:"valid"`
}

// Payload e' l'unione di tutti i campi che sonde puo' ricevere. I numeri sono
// puntatori per distinguere uno zero reale da un campo assente.
type Payload struct {
	ClientID  string  `json:"client-id"`
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Timestamp float64 `json:"timestamp"`

	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`

	UltrasonicCM *float64 `json:"ultrasonic_cm"`
	TofCM        *float64 `json:"tof_cm"`
	WaterLevelCM *float64 `json:"water_level_cm"`
	Value        *float64 `json:"value"`

	RSSI *int   `json:"rssi"`
	Net  string `json:"net"`

	PMS     *PMS     `json:"pms"`
	DHT22   *DHT22   `json:"dht22"`
	BME280  *BME280  `json:"bme280"`
	Virtual *Virtual `json:"virtual"`
	GPS     *GPS     `json:"gps"`
}

// Reading e' la forma normalizzata di un Payload, pronta per lo storage.
type Reading struct {
	DeviceID string
	Label    string
	Group    string
	Topic    string
	Kind     Kind
	TS       time.Time

	WaterLevelCM *float64
	UltrasonicCM *float64
	TofCM        *float64
	Value        *float64

	PM1, PM25, PM10              *float64
	TempPMS, HumPMS              *float64
	TempDHT, HumDHT              *float64
	TempBME, HumBME, PressBME    *float64
	TempVirt, HumVirt, PressVirt *float64

	Lat, Lon *float64
	RSSI     *int
	Net      string
}

// Parse decodifica un payload MQTT grezzo e lo normalizza in una Reading.
func Parse(topic string, raw []byte) (Reading, error) {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Reading{}, err
	}
	return Normalize(topic, p), nil
}

// Normalize converte un Payload gia' decodificato in una Reading. Il nome
// mostrato sul web e' sempre il campo "label"; se manca si usa l'id ricavato
// dal topic.
func Normalize(topic string, p Payload) Reading {
	group, dev := ParseTopic(topic)
	if dev == "" {
		dev = firstNonEmpty(p.ClientID, p.ID)
	}
	if dev == "" {
		dev = "sconosciuto"
	}
	label := strings.TrimSpace(p.Label)
	if label == "" {
		label = dev
	}

	ts := time.Now().UTC()
	if p.Timestamp > 0 {
		sec := int64(p.Timestamp)
		nsec := int64((p.Timestamp - float64(sec)) * 1e9)
		ts = time.Unix(sec, nsec).UTC()
	}

	r := Reading{
		DeviceID:     dev,
		Label:        label,
		Group:        group,
		Topic:        topic,
		TS:           ts,
		WaterLevelCM: p.WaterLevelCM,
		UltrasonicCM: p.UltrasonicCM,
		TofCM:        p.TofCM,
		Value:        p.Value,
		Lat:          p.Lat,
		Lon:          p.Lon,
		RSSI:         p.RSSI,
		Net:          p.Net,
	}

	air := false
	if p.PMS != nil {
		air = true
		r.PM1, r.PM25, r.PM10 = p.PMS.PM1, p.PMS.PM25, p.PMS.PM10
		r.TempPMS, r.HumPMS = p.PMS.Temperature, p.PMS.Humidity
	}
	if p.DHT22 != nil {
		air = true
		r.TempDHT, r.HumDHT = p.DHT22.Temperature, p.DHT22.Humidity
	}
	if p.BME280 != nil {
		air = true
		r.TempBME, r.HumBME, r.PressBME = p.BME280.Temperature, p.BME280.Humidity, p.BME280.Pressure
	}
	if p.Virtual != nil {
		air = true
		r.TempVirt, r.HumVirt, r.PressVirt = p.Virtual.Temperature, p.Virtual.Humidity, p.Virtual.Pressure
	}
	if p.GPS != nil {
		if r.Lat == nil {
			r.Lat = p.GPS.Lat
		}
		if r.Lon == nil {
			r.Lon = p.GPS.Lon
		}
	}

	level := r.UltrasonicCM != nil || r.TofCM != nil || r.WaterLevelCM != nil || r.Value != nil
	switch {
	case level && air:
		r.Kind = KindMixed
	case air:
		r.Kind = KindAir
	case level:
		r.Kind = KindLevel
	default:
		r.Kind = KindUnknown
	}

	if r.WaterLevelCM == nil {
		switch {
		case r.UltrasonicCM != nil:
			r.WaterLevelCM = r.UltrasonicCM
		case r.Value != nil:
			r.WaterLevelCM = r.Value
		}
	}
	if r.Value == nil {
		r.Value = r.WaterLevelCM
	}

	return r
}

// ParseTopic ricava gruppo e id dispositivo da un topic MQTT.
//
// Lo schema canonico e' "{root}/{device_id}" con "/data" opzionale. Eventuali
// livelli extra finiscono nel gruppo e l'ultimo segmento utile e' l'id, quindi
// sia "421342415/Rio_Molini_0" sia il vecchio
// "421342415/ID_1_Vigili_VillaLagarina_TN/Rio_Molini_0/data" indicano lo
// stesso dispositivo.
func ParseTopic(topic string) (group, deviceID string) {
	parts := strings.Split(topic, "/")
	clean := make([]string, 0, len(parts))
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) > 0 && strings.EqualFold(clean[len(clean)-1], "data") {
		clean = clean[:len(clean)-1]
	}
	if len(clean) == 0 {
		return "", ""
	}
	deviceID = clean[len(clean)-1]
	if len(clean) > 1 {
		group = strings.Join(clean[:len(clean)-1], "/")
	}
	return group, deviceID
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
