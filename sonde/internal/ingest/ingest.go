// Package ingest normalizza i messaggi MQTT, li salva e li propaga alla dashboard.
package ingest

import (
	"encoding/json"
	"log"

	"sonde/internal/hub"
	"sonde/internal/store"
	"sonde/internal/telemetry"
)

// Ingestor collega il broker MQTT allo store e all'hub SSE.
type Ingestor struct {
	store *store.Store
	hub   *hub.Hub
	log   *log.Logger
}

// New crea un Ingestor.
func New(s *store.Store, h *hub.Hub, logger *log.Logger) *Ingestor {
	return &Ingestor{store: s, hub: h, log: logger}
}

// Handle salva una lettura e la pubblica via SSE.
func (ig *Ingestor) Handle(topic string, payload []byte) {
	r, err := telemetry.Parse(topic, payload)
	if err != nil {
		ig.log.Printf("payload MQTT non valido su %q: %v", topic, err)
		return
	}
	if r.Kind == telemetry.KindUnknown {
		return
	}
	if err := ig.store.Insert(r); err != nil {
		ig.log.Printf("insert fallito (%s): %v", r.DeviceID, err)
		return
	}
	if data, err := json.Marshal(store.FromReading(r)); err == nil {
		ig.hub.Broadcast("reading", data)
	}
}
