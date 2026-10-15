// Package api espone l'API HTTP e la dashboard statica.
package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sonde/internal/export"
	"sonde/internal/hub"
	"sonde/internal/store"
)

// Server raggruppa le dipendenze degli handler HTTP.
type Server struct {
	store *store.Store
	hub   *hub.Hub
	web   fs.FS
	log   *log.Logger
}

// New crea il server HTTP.
func New(s *store.Store, h *hub.Hub, web fs.FS, logger *log.Logger) *Server {
	return &Server{store: s, hub: h, web: web, log: logger}
}

// Handler costruisce il router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/latest", s.handleLatest)
	mux.HandleFunc("/api/readings", s.handleReadings)
	mux.HandleFunc("/api/stats", s.handleStats)
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
	devices, err := s.store.ListDevices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if devices == nil {
		devices = []store.Device{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
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
