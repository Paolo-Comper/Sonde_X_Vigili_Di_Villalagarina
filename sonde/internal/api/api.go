// Package api espone l'API HTTP e la dashboard statica.
package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"sonde/internal/broker"
	"sonde/internal/config"
	"sonde/internal/export"
	"sonde/internal/hub"
	"sonde/internal/store"
	"sonde/internal/topology"
)

// MQTTController permette di riapplicare la configurazione a caldo.
type MQTTController interface {
	Apply(broker.Config) error
	Status() broker.Status
}

// Server raggruppa le dipendenze degli handler HTTP.
type Server struct {
	store        *store.Store
	hub          *hub.Hub
	topo         topology.Topology
	topoPath     string
	topoMu       sync.RWMutex
	settingsPath string
	mqttCfg      config.MQTT
	cfgMu        sync.RWMutex
	mqtt         MQTTController
	web          fs.FS
	log          *log.Logger
}

// New crea il server HTTP. topoPath e' il file su cui salvare le modifiche
// alla topologia (vuoto = solo in memoria).
func New(s *store.Store, h *hub.Hub, topo topology.Topology, topoPath, settingsPath string, mqttCfg config.MQTT, mqtt MQTTController, web fs.FS, logger *log.Logger) *Server {
	return &Server{
		store:        s,
		hub:          h,
		topo:         topo,
		topoPath:     topoPath,
		settingsPath: settingsPath,
		mqttCfg:      mqttCfg,
		mqtt:         mqtt,
		web:          web,
		log:          logger,
	}
}

// Handler costruisce il router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/latest", s.handleLatest)
	mux.HandleFunc("/api/readings", s.handleReadings)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/topology", s.handleTopology)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/schema", s.handleSchema)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/clear-exported", s.handleClearExported)
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.Handle("/", http.FileServer(http.FS(s.web)))
	return mux
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices, err := s.store.ListDevices()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if devices == nil {
			devices = []store.Device{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
	case http.MethodPost:
		s.addDevice(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("metodo non consentito"))
	}
}

type deviceInput struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Lat   *float64 `json:"lat"`
	Lon   *float64 `json:"lon"`
}

// addDevice registra (o aggiorna) un dispositivo anche prima della prima
// misura, cosi' compare subito in elenco, mappa e grafo.
func (s *Server) addDevice(w http.ResponseWriter, r *http.Request) {
	var in deviceInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("JSON non valido: %w", err))
		return
	}

	id := strings.TrimSpace(in.ID)
	label := strings.TrimSpace(in.Label)
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id dispositivo obbligatorio"))
		return
	}
	if strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("l'id non puo' contenere '/'"))
		return
	}
	if label == "" {
		label = id
	}
	if (in.Lat == nil) != (in.Lon == nil) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("latitudine e longitudine vanno indicate insieme"))
		return
	}
	if in.Lat != nil && (*in.Lat < -90 || *in.Lat > 90) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("latitudine fuori intervallo"))
		return
	}
	if in.Lon != nil && (*in.Lon < -180 || *in.Lon > 180) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("longitudine fuori intervallo"))
		return
	}

	s.cfgMu.RLock()
	cfg := s.mqttCfg
	s.cfgMu.RUnlock()

	now := time.Now().Unix()
	d := store.Device{
		ID:        id,
		Label:     label,
		Group:     cfg.Group(),
		Topic:     cfg.DeviceTopic(id),
		Lat:       in.Lat,
		Lon:       in.Lon,
		FirstSeen: now,
		LastSeen:  now,
	}
	if err := s.store.UpsertDevice(d); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if data, err := json.Marshal(d); err == nil {
		s.hub.Broadcast("devices", data)
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Latest()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if data == nil {
		data = []store.Measurement{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) handleReadings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	data, err := s.store.Readings(q.Get("device"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if data == nil {
		data = []store.Measurement{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleTopology legge o aggiorna i collegamenti di affluenza configurati.
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.topoMu.RLock()
		t := s.topo
		s.topoMu.RUnlock()
		writeJSON(w, http.StatusOK, t)
	case http.MethodPut, http.MethodPost:
		s.updateTopology(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("metodo non consentito"))
	}
}

func (s *Server) updateTopology(w http.ResponseWriter, r *http.Request) {
	var t topology.Topology
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("JSON non valido: %w", err))
		return
	}

	links := make([]topology.Link, 0, len(t.Links))
	seen := make(map[string]bool, len(t.Links))
	for _, l := range t.Links {
		l.From = strings.TrimSpace(l.From)
		l.To = strings.TrimSpace(l.To)
		if l.From == "" || l.To == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("ogni collegamento deve avere 'from' e 'to'"))
			return
		}
		if strings.EqualFold(l.From, l.To) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("una sonda non puo' collegarsi a se stessa (%s)", l.From))
			return
		}
		key := strings.ToLower(l.From) + "\x00" + strings.ToLower(l.To)
		if seen[key] {
			continue
		}
		seen[key] = true
		links = append(links, l)
	}
	t.Links = links

	if s.topoPath != "" {
		if err := topology.Save(s.topoPath, t); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("salvataggio topologia: %w", err))
			return
		}
	}

	s.topoMu.Lock()
	s.topo = t
	s.topoMu.Unlock()

	if data, err := json.Marshal(t); err == nil {
		s.hub.Broadcast("topology", data)
	}
	writeJSON(w, http.StatusOK, t)
}

type mqttView struct {
	Broker      string `json:"broker"`
	Prefix      string `json:"prefix"`
	Topic       string `json:"topic"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
}

type settingsView struct {
	MQTT   mqttView      `json:"mqtt"`
	Status broker.Status `json:"status"`
}

func (s *Server) currentMQTT() config.MQTT {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.mqttCfg
}

func (s *Server) settingsView(cfg config.MQTT) settingsView {
	v := settingsView{
		MQTT: mqttView{
			Broker:      cfg.Broker,
			Prefix:      cfg.Prefix,
			Topic:       cfg.Topic,
			Username:    cfg.Username,
			PasswordSet: cfg.Password != "",
		},
	}
	if s.mqtt != nil {
		v.Status = s.mqtt.Status()
	}
	if v.Status.Broker == "" {
		v.Status.Broker = cfg.BrokerURL()
	}
	if v.Status.Topic == "" {
		v.Status.Topic = cfg.Topic
	}
	return v
}

// handleSettings legge o aggiorna broker, prefisso e credenziali MQTT.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.settingsView(s.currentMQTT()))
	case http.MethodPut, http.MethodPost:
		s.updateSettings(w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("metodo non consentito"))
	}
}

type settingsInput struct {
	Broker        string  `json:"broker"`
	Prefix        string  `json:"prefix"`
	Username      *string `json:"username"`
	Password      string  `json:"password"`
	ClearPassword bool    `json:"clear_password"`
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("JSON non valido: %w", err))
		return
	}

	cfg := s.currentMQTT()
	if b := strings.TrimSpace(in.Broker); b != "" {
		cfg.Broker = b
	}
	if p := strings.TrimSpace(in.Prefix); p != "" {
		cfg.Prefix = p
	} else {
		writeError(w, http.StatusBadRequest, fmt.Errorf("prefisso topic obbligatorio"))
		return
	}
	if in.Username != nil {
		cfg.Username = strings.TrimSpace(*in.Username)
	}
	if in.ClearPassword {
		cfg.Password = ""
	} else if in.Password != "" {
		cfg.Password = in.Password
	}
	cfg = cfg.Normalized()

	if err := cfg.Save(s.settingsPath); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("salvataggio impostazioni: %w", err))
		return
	}

	s.cfgMu.Lock()
	s.mqttCfg = cfg
	s.cfgMu.Unlock()

	var applyErr error
	if s.mqtt != nil {
		applyErr = s.mqtt.Apply(broker.Config{
			URL:      cfg.BrokerURL(),
			Topic:    cfg.Topic,
			Username: cfg.Username,
			Password: cfg.Password,
		})
	}

	view := s.settingsView(cfg)
	if data, err := json.Marshal(view); err == nil {
		s.hub.Broadcast("settings", data)
	}
	if applyErr != nil {
		writeJSON(w, http.StatusBadGateway, view)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	cols, err := s.store.Schema()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols})
}

// handleQuery esegue una SELECT e restituisce le righe come JSON (anteprima).
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	sql := querySQL(r)
	if err := export.ValidateSelect(sql); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.QueryRows(sql)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	table, err := export.Materialize(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	recs := table.Records(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"columns": table.Columns,
		"count":   len(table.Rows),
		"rows":    recs,
	})
}

// handleExport esegue la query e scarica il risultato come CSV, JSON o ZIP.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "csv"
	}
	mark := r.URL.Query().Get("mark") == "1" || r.URL.Query().Get("mark") == "true"

	sql := querySQL(r)
	if err := export.ValidateSelect(sql); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rows, err := s.store.QueryRows(sql)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	table, err := export.Materialize(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	base := "sonde_export_" + time.Now().Format("20060102_150405")
	var (
		content     []byte
		contentType string
		ext         string
	)
	switch format {
	case "csv":
		var buf bytes.Buffer
		if err := table.WriteCSV(&buf); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		content, contentType, ext = buf.Bytes(), "text/csv; charset=utf-8", "csv"
	case "json":
		var buf bytes.Buffer
		if err := table.WriteJSON(&buf); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		content, contentType, ext = buf.Bytes(), "application/json; charset=utf-8", "json"
	case "zip":
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		csvFile, err := zw.Create(base + ".csv")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := table.WriteCSV(csvFile); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		jsonFile, err := zw.Create(base + ".json")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := table.WriteJSON(jsonFile); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := zw.Close(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		content, contentType, ext = buf.Bytes(), "application/zip", "zip"
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("formato non supportato: %q", format))
		return
	}

	marked := 0
	if mark && len(table.IDs) > 0 {
		n, err := s.store.MarkExported(table.IDs)
		if err != nil {
			s.log.Printf("mark exported: %v", err)
		}
		marked = int(n)
		if data, err := json.Marshal(map[string]any{"marked": marked}); err == nil {
			s.hub.Broadcast("exported", data)
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.%s\"", base, ext))
	w.Header().Set("X-Exported-Count", strconv.Itoa(marked))
	w.Write(content)
}

// handleClearExported elimina le misure gia' esportate.
func (s *Server) handleClearExported(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("metodo non consentito"))
		return
	}
	n, err := s.store.ClearExported()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.hub.Broadcast("exported", []byte(`{"marked":0}`))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
}

// handleStream invia gli aggiornamenti live via Server-Sent Events.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming non supportato"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	fmt.Fprint(w, "event: hello\ndata: {}\n\n")
	flusher.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data)
			flusher.Flush()
		}
	}
}

const defaultExportSQL = "SELECT * FROM measurements ORDER BY ts ASC, id ASC"

func querySQL(r *http.Request) string {
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		if q := strings.TrimSpace(r.FormValue("sql")); q != "" {
			return q
		}
	}
	if q := strings.TrimSpace(r.URL.Query().Get("sql")); q != "" {
		return q
	}
	return defaultExportSQL
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
