package store

import (
	"path/filepath"
	"testing"
	"time"

	"argine/internal/telemetry"
)

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func levelReading() telemetry.Reading {
	return telemetry.Reading{
		DeviceID:     "Rio_Molini_0",
		Label:        "Rio Molini 0",
		Group:        "421342415",
		Topic:        "421342415/Rio_Molini_0",
		Kind:         telemetry.KindLevel,
		TS:           time.Unix(1000, 0).UTC(),
		WaterLevelCM: f(123.45),
		UltrasonicCM: f(123.45),
		TofCM:        f(122.80),
		Value:        f(123.45),
		Lat:          f(45.915771),
		Lon:          f(11.024346),
		RSSI:         i(25),
		Net:          "LTE",
	}
}

func TestUpsertDeviceKeepsManualLabelAndPosition(t *testing.T) {
	s := newTestStore(t)

	lat, lon := 45.9, 11.0
	if err := s.UpsertDevice(Device{
		ID: "Rio_Molini_0", Label: "Rio Molini 0", Group: "421342415",
		Topic: "421342415/Rio_Molini_0", Lat: &lat, Lon: &lon,
	}); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}

	// Una misura senza label e senza GPS non deve cancellare i dati manuali.
	r := levelReading()
	r.Label = r.DeviceID
	r.Lat, r.Lon = nil, nil
	if err := s.Insert(r); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	devices, err := s.ListDevices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("ListDevices = %+v err:%v", devices, err)
	}
	d := devices[0]
	if d.Label != "Rio Molini 0" {
		t.Errorf("label manuale sovrascritta: %q", d.Label)
	}
	if d.Lat == nil || *d.Lat != lat || d.Lon == nil || *d.Lon != lon {
		t.Errorf("posizione manuale persa: %+v", d)
	}

	// Una label esplicita nel payload invece aggiorna il nome.
	r.Label = "Rio Molini nuovo"
	if err := s.Insert(r); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	devices, _ = s.ListDevices()
	if devices[0].Label != "Rio Molini nuovo" {
		t.Errorf("label esplicita non applicata: %q", devices[0].Label)
	}
}

func airReading() telemetry.Reading {
	return telemetry.Reading{
		DeviceID: "rpi-zero-1",
		Label:    "Sensore aria 1",
		Group:    "argine",
		Topic:    "argine/rpi-zero-1/data",
		Kind:     telemetry.KindAir,
		TS:       time.Unix(2000, 0).UTC(),
		PM1:      f(4), PM25: f(10), PM10: f(15),
		TempPMS: f(9.1), HumPMS: f(63.3),
		TempBME: f(4.3), HumBME: f(74.2), PressBME: f(999.57),
		TempVirt: f(4.6), HumVirt: f(76.5), PressVirt: f(999.62),
		Lat: f(43.72386), Lon: f(12.63765),
	}
}

func TestInsertAndQuery(t *testing.T) {
	s := newTestStore(t)

	if err := s.Insert(levelReading()); err != nil {
		t.Fatalf("insert level: %v", err)
	}
	if err := s.Insert(airReading()); err != nil {
		t.Fatalf("insert air: %v", err)
	}
	// Una seconda misura di livello sullo stesso dispositivo.
	r := levelReading()
	r.TS = time.Unix(1500, 0).UTC()
	r.WaterLevelCM = f(130)
	r.Value = f(130)
	if err := s.Insert(r); err != nil {
		t.Fatalf("insert level 2: %v", err)
	}

	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(devices))
	}
	if devices[0].Readings+devices[1].Readings != 3 {
		t.Errorf("reading counts = %d+%d, want total 3", devices[0].Readings, devices[1].Readings)
	}

	latest, err := s.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("got %d latest, want 2", len(latest))
	}
	for _, m := range latest {
		if m.DeviceID == "Rio_Molini_0" {
			if m.WaterLevel == nil || *m.WaterLevel != 130 {
				t.Errorf("latest level = %v, want 130", m.WaterLevel)
			}
			if m.DeviceLabel != "Rio Molini 0" {
				t.Errorf("label = %q", m.DeviceLabel)
			}
		}
	}

	only, err := s.Readings("rpi-zero-1", 10)
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	if len(only) != 1 || only[0].PM25 == nil || *only[0].PM25 != 10 {
		t.Errorf("readings filter wrong: %+v", only)
	}
}

func TestMarkExportedAndStats(t *testing.T) {
	s := newTestStore(t)
	if err := s.Insert(levelReading()); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(airReading()); err != nil {
		t.Fatal(err)
	}

	st, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Readings != 2 || st.Exported != 0 || st.Pending != 2 {
		t.Fatalf("stats = %+v", st)
	}

	if _, err := s.MarkExported([]int64{1}); err != nil {
		t.Fatalf("MarkExported: %v", err)
	}
	st, _ = s.Stats()
	if st.Exported != 1 || st.Pending != 1 {
		t.Errorf("after marking: %+v", st)
	}
}

func TestSchema(t *testing.T) {
	s := newTestStore(t)
	cols, err := s.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	found := false
	for _, c := range cols {
		if c.Name == "exported" {
			found = true
		}
	}
	if !found {
		t.Errorf("exported column missing from schema: %+v", cols)
	}
}

func TestQueryRowsRollbackIsSafe(t *testing.T) {
	s := newTestStore(t)
	if err := s.Insert(levelReading()); err != nil {
		t.Fatal(err)
	}
	rows, err := s.QueryRows("SELECT id, device_label FROM measurements")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()
	if count != 1 {
		t.Errorf("got %d rows, want 1", count)
	}
}
