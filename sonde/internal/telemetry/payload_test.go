package telemetry

import (
	"testing"
	"time"
)

func TestParseTopic(t *testing.T) {
	cases := []struct {
		topic  string
		group  string
		device string
	}{
		{"421342415/Rio_Molini_0", "421342415", "Rio_Molini_0"},
		{"421342415/Rio_Molini_0/data", "421342415", "Rio_Molini_0"},
		{"421342415/ID_1_Vigili_VillaLagarina_TN/Rio_Molini_0", "421342415/ID_1_Vigili_VillaLagarina_TN", "Rio_Molini_0"},
		{"sonde/rpi-zero-1/data", "sonde", "rpi-zero-1"},
		{"  //solo_device//  ", "", "solo_device"},
		{"", "", ""},
	}
	for _, c := range cases {
		g, d := ParseTopic(c.topic)
		if g != c.group || d != c.device {
			t.Errorf("ParseTopic(%q) = (%q,%q), want (%q,%q)", c.topic, g, d, c.group, c.device)
		}
	}
}

func TestNormalizeFlatLevel(t *testing.T) {
	raw := []byte(`{
		"client-id":"id_TN_VL_0",
		"label":"Rio Molini 0",
		"lat":45.915771,"lon":11.024346,
		"ultrasonic_cm":123.45,"tof_cm":122.80,
		"rssi":25,"net":"LTE"
	}`)

	r, err := Parse("421342415/Rio_Molini_0", raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if r.DeviceID != "Rio_Molini_0" {
		t.Errorf("DeviceID = %q", r.DeviceID)
	}
	if r.Label != "Rio Molini 0" {
		t.Errorf("Label = %q", r.Label)
	}
	if r.Kind != KindLevel {
		t.Errorf("Kind = %q, want %q", r.Kind, KindLevel)
	}
	if r.WaterLevelCM == nil || *r.WaterLevelCM != 123.45 {
		t.Errorf("WaterLevelCM = %v, want 123.45", r.WaterLevelCM)
	}
	if r.Lat == nil || *r.Lat != 45.915771 {
		t.Errorf("Lat = %v", r.Lat)
	}
	if r.RSSI == nil || *r.RSSI != 25 {
		t.Errorf("RSSI = %v", r.RSSI)
	}
}

func TestNormalizeNestedAir(t *testing.T) {
	raw := []byte(`{
		"timestamp":1771815991.5,
		"pms":{"pm1":4,"pm25":10,"pm10":15,"temperature":9.1,"humidity":63.3},
		"bme280":{"temperature":4.3,"humidity":74.2,"pressure":999.57},
		"virtual":{"temperature":4.6,"humidity":76.5,"pressure":999.62},
		"gps":{"lat":43.72386,"lon":12.63765,"alt":472.0,"sats":7,"valid":true}
	}`)

	r, err := Parse("sonde/rpi-zero-1/data", raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if r.DeviceID != "rpi-zero-1" {
		t.Errorf("DeviceID = %q", r.DeviceID)
	}
	// Senza label nel payload si usa l'id come nome mostrato.
	if r.Label != "rpi-zero-1" {
		t.Errorf("Label = %q, want device id fallback", r.Label)
	}
	if r.Kind != KindAir {
		t.Errorf("Kind = %q, want %q", r.Kind, KindAir)
	}
	if r.PM25 == nil || *r.PM25 != 10 {
		t.Errorf("PM25 = %v", r.PM25)
	}
	if r.WaterLevelCM != nil {
		t.Errorf("WaterLevelCM = %v, want nil for an air-only device", r.WaterLevelCM)
	}
	want := time.Unix(1771815991, 500000000).UTC()
	if !r.TS.Equal(want) {
		t.Errorf("TS = %v, want %v", r.TS, want)
	}
}

func TestNormalizeMixed(t *testing.T) {
	raw := []byte(`{
		"label":"Stazione mista",
		"ultrasonic_cm":50,
		"pms":{"pm1":1,"pm25":2,"pm10":3}
	}`)
	r, err := Parse("sonde/mista", raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if r.Kind != KindMixed {
		t.Errorf("Kind = %q, want %q", r.Kind, KindMixed)
	}
	if r.WaterLevelCM == nil || *r.WaterLevelCM != 50 {
		t.Errorf("WaterLevelCM = %v", r.WaterLevelCM)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := Parse("sonde/x", []byte("not json")); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
