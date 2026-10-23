package export

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"argine/internal/store"
	"argine/internal/telemetry"
)

func TestValidateSelect(t *testing.T) {
	ok := []string{
		"SELECT * FROM measurements",
		"SELECT id, device_label FROM measurements WHERE exported = 0",
		"select count(*) from measurements;",
		"WITH recent AS (SELECT * FROM measurements ORDER BY ts DESC LIMIT 10) SELECT * FROM recent",
	}
	for _, q := range ok {
		if err := ValidateSelect(q); err != nil {
			t.Errorf("ValidateSelect(%q) = %v, want nil", q, err)
		}
	}

	bad := []string{
		"",
		"DELETE FROM measurements",
		"UPDATE measurements SET exported = 1",
		"DROP TABLE measurements",
		"SELECT 1; DROP TABLE measurements",
		"INSERT INTO measurements(id) VALUES (1)",
		"PRAGMA table_info(measurements)",
	}
	for _, q := range bad {
		if err := ValidateSelect(q); err == nil {
			t.Errorf("ValidateSelect(%q) = nil, want error", q)
		}
	}
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "exp.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seed(t *testing.T, s *store.Store) {
	t.Helper()
	lat := 45.9
	cm := 123.45
	r := telemetry.Reading{
		DeviceID:     "Rio_Molini_0",
		Label:        "Rio Molini 0",
		Topic:        "421342415/Rio_Molini_0",
		Kind:         telemetry.KindLevel,
		TS:           time.Unix(1000, 0).UTC(),
		Lat:          &lat,
		WaterLevelCM: &cm,
		UltrasonicCM: &cm,
	}
	if err := s.Insert(r); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestMaterializeCSV(t *testing.T) {
	s := newStore(t)
	seed(t, s)

	rows, err := s.QueryRows("SELECT id, device_label, water_level_cm FROM measurements")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	table, err := Materialize(rows)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(table.IDs) != 1 || table.IDs[0] != 1 {
		t.Errorf("ids = %v, want [1]", table.IDs)
	}

	var sb strings.Builder
	if err := table.WriteCSV(&sb); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := sb.String()
	if !strings.HasPrefix(out, "id,device_label,water_level_cm\n") {
		t.Errorf("unexpected header: %q", out)
	}
	if !strings.Contains(out, "Rio Molini 0") || !strings.Contains(out, "123.45") {
		t.Errorf("unexpected body: %q", out)
	}
}

func TestMaterializeJSON(t *testing.T) {
	s := newStore(t)
	seed(t, s)

	rows, err := s.QueryRows("SELECT id, device_label, exported FROM measurements")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	table, err := Materialize(rows)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	var sb strings.Builder
	if err := table.WriteJSON(&sb); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, `"device_label": "Rio Molini 0"`) {
		t.Errorf("unexpected JSON: %q", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("JSON must be an array: %q", out)
	}
}

func TestRecords(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	rows, err := s.QueryRows("SELECT id, device_label FROM measurements")
	if err != nil {
		t.Fatal(err)
	}
	table, err := Materialize(rows)
	if err != nil {
		t.Fatal(err)
	}
	recs := table.Records(10)
	if len(recs) != 1 || recs[0]["device_label"] != "Rio Molini 0" {
		t.Errorf("records = %+v", recs)
	}
}
