package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"sonde/internal/broker"
	"sonde/internal/config"
	"sonde/internal/hub"
	"sonde/internal/store"
	"sonde/internal/telemetry"
	"sonde/internal/topology"
)

type fakeMQTT struct {
	applied broker.Config
	status  broker.Status
	err     error
}

func (f *fakeMQTT) Apply(cfg broker.Config) error {
	f.applied = cfg
	f.status = broker.Status{Broker: cfg.URL, Topic: cfg.Topic, Connected: f.err == nil}
	if f.err != nil {
		f.status.Error = f.err.Error()
	}
	return f.err
}

func (f *fakeMQTT) Status() broker.Status { return f.status }

func newTestServer(t *testing.T) (*Server, *store.Store, *fakeMQTT) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	h := hub.New()
	topoPath := filepath.Join(t.TempDir(), "topology.json")
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	mq := &fakeMQTT{status: broker.Status{Connected: true, Topic: "421342415/#"}}
	cfg := config.MQTT{
		Broker: "broker.hivemq.com:1883", Prefix: "421342415",
		Username: "utente", Password: "segreta",
	}.Normalized()
	srv := New(s, h, topology.Topology{}, topoPath, settingsPath, cfg, mq, fstest.MapFS{}, log.New(io.Discard, "", 0))

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
	return srv, s, mq
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
	srv, _, _ := newTestServer(t)
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
	srv, _, _ := newTestServer(t)

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
	srv, s, _ := newTestServer(t)

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
	srv, _, _ := newTestServer(t)
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

func TestTopologyUpdatePersists(t *testing.T) {
	srv, _, _ := newTestServer(t)

	body := `{"links":[{"from":"Rio Molini 0","to":"Rio Piazzo 0"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/topology", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", rec.Code, rec.Body.String())
	}

	if len(srv.topo.Links) != 1 || srv.topo.Links[0].To != "Rio Piazzo 0" {
		t.Fatalf("topologia in memoria = %+v", srv.topo)
	}
	data, err := os.ReadFile(srv.topoPath)
	if err != nil {
		t.Fatalf("file topologia: %v", err)
	}
	if !strings.Contains(string(data), "Rio Piazzo 0") {
		t.Errorf("file topologia = %s", data)
	}

	res, out := do(t, srv, "/api/topology")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", res.StatusCode)
	}
	if !strings.Contains(string(out), "Rio Piazzo 0") {
		t.Errorf("GET topologia = %s", out)
	}

	// Autocollegamento rifiutato.
	bad := `{"links":[{"from":"A","to":"A"}]}`
	req = httptest.NewRequest(http.MethodPut, "/api/topology", strings.NewReader(bad))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PUT autocollegamento status = %d, want 400", rec.Code)
	}
}

func TestSettingsEndpoint(t *testing.T) {
	srv, _, mq := newTestServer(t)

	res, body := do(t, srv, "/api/settings")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "segreta") {
		t.Errorf("la password non deve essere esposta: %s", body)
	}
	var got settingsView
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got.MQTT.Prefix != "421342415" || got.MQTT.Topic != "421342415/#" || !got.MQTT.PasswordSet {
		t.Errorf("settings = %+v", got)
	}

	payload := `{"broker":"test:1883","prefix":"nuovo/","username":"u2","password":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", rec.Code, rec.Body.String())
	}
	var updated settingsView
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json: %v", err)
	}
	if updated.MQTT.Topic != "nuovo/#" {
		t.Errorf("topic = %q", updated.MQTT.Topic)
	}
	if !updated.MQTT.PasswordSet {
		t.Error("una password vuota deve mantenere quella esistente")
	}
	if mq.applied.Topic != "nuovo/#" || mq.applied.URL != "tcp://test:1883" || mq.applied.Username != "u2" {
		t.Errorf("config applicata = %+v", mq.applied)
	}
	saved, ok, err := config.Load(srv.settingsPath)
	if err != nil || !ok || saved.Prefix != "nuovo" {
		t.Errorf("config persistita = %+v ok:%v err:%v", saved, ok, err)
	}

	// Prefisso vuoto rifiutato.
	req = httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"prefix":""}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("prefisso vuoto status = %d, want 400", rec.Code)
	}
}

func TestAddDeviceEndpoint(t *testing.T) {
	srv, _, _ := newTestServer(t)

	body := `{"id":"Rio_Nuovo_0","label":"Rio Nuovo 0","lat":45.9,"lon":11.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/devices", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	var d store.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("json: %v", err)
	}
	if d.ID != "Rio_Nuovo_0" || d.Label != "Rio Nuovo 0" || d.Topic != "421342415/Rio_Nuovo_0" {
		t.Errorf("device = %+v", d)
	}

	res, out := do(t, srv, "/api/devices")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", res.StatusCode)
	}
	var list struct {
		Devices []store.Device `json:"devices"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatalf("json: %v", err)
	}
	found := false
	for _, dev := range list.Devices {
		if dev.ID == "Rio_Nuovo_0" {
			found = true
		}
	}
	if !found {
		t.Errorf("dispositivo aggiunto non presente: %+v", list.Devices)
	}

	// ID con slash rifiutato.
	req = httptest.NewRequest(http.MethodPost, "/api/devices", strings.NewReader(`{"id":"a/b"}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("id con slash status = %d, want 400", rec.Code)
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv, _, _ := newTestServer(t)
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
