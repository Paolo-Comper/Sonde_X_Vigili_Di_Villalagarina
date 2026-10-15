package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"sonde/internal/hub"
	"sonde/internal/store"
	"sonde/internal/telemetry"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	h := hub.New()
	srv := New(s, h, fstest.MapFS{}, log.New(io.Discard, "", 0))

	cm := 123.45
	lat := 45.915771
	r := telemetry.Reading{
		DeviceID:     "Rio_Molini_0",
		Label:        "Rio Molini 0",
		Topic:        "421342415/Rio_Molini_0",
		Kind:         telemetry.KindLevel,
		TS:           time.Unix(1000, 0).UTC(),
		WaterLevelCM: &cm,
		UltrasonicCM: &cm,
		Lat:          &lat,
	}
	if err := s.Insert(r); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return srv, s
}

func do(t *testing.T, srv *Server, path string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res, body
}

func TestDevicesEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	res, body := do(t, srv, "/api/devices")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var out struct {
		Devices []store.Device `json:"devices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v (%s)", err, body)
	}
	if len(out.Devices) != 1 || out.Devices[0].Label != "Rio Molini 0" {
		t.Errorf("devices = %+v", out.Devices)
	}
}

func TestQueryEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	q := url.QueryEscape("SELECT id, device_label FROM measurements WHERE exported = 0")
	res, body := do(t, srv, "/api/query?sql="+q)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.StatusCode, body)
	}
	var out struct {
		Count int              `json:"count"`
		Rows  []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Count != 1 || out.Rows[0]["device_label"] != "Rio Molini 0" {
		t.Errorf("query result = %+v", out)
	}

	// Un'istruzione di scrittura deve essere rifiutata.
	res, _ = do(t, srv, "/api/query?sql="+url.QueryEscape("DELETE FROM measurements"))
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("DELETE status = %d, want 400", res.StatusCode)
	}
}

func TestExportCSVAndMark(t *testing.T) {
	srv, s := newTestServer(t)

	q := url.QueryEscape("SELECT * FROM measurements WHERE exported = 0 ORDER BY id")
	res, body := do(t, srv, "/api/export?format=csv&mark=1&sql="+q)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type = %q", ct)
	}
	if !strings.Contains(string(body), "Rio Molini 0") {
		t.Errorf("csv body = %s", body)
	}
	if got := res.Header.Get("X-Exported-Count"); got != "1" {
		t.Errorf("X-Exported-Count = %q, want 1", got)
	}

	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Exported != 1 || st.Pending != 0 {
		t.Errorf("stats after export = %+v", st)
	}
}

func TestExportZIP(t *testing.T) {
	srv, _ := newTestServer(t)
	res, body := do(t, srv, "/api/export?format=zip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("content-type = %q", ct)
	}
	if len(body) < 4 || string(body[:2]) != "PK" {
		t.Errorf("not a zip archive")
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	res, body := do(t, srv, "/api/stats")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var st store.Stats
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("json: %v", err)
	}
	if st.Readings != 1 || st.Devices != 1 {
		t.Errorf("stats = %+v", st)
	}
}
