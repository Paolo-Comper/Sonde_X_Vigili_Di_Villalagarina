package main

import (
	"io"
	"log"
	"net"
	"path/filepath"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"

	"argine/internal/broker"
	"argine/internal/hub"
	"argine/internal/ingest"
	"argine/internal/sim"
	"argine/internal/store"
)

// startBroker avvia un broker MQTT in-process su una porta libera.
func startBroker(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	srv := mochi.New(nil)
	if err := srv.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if err := srv.AddListener(listeners.NewTCP(listeners.Config{ID: "tcp", Address: addr})); err != nil {
		t.Fatalf("listener: %v", err)
	}
	go func() {
		if err := srv.Serve(); err != nil {
			t.Logf("broker serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Close() })

	// Attende che la porta risponda.
	for i := 0; i < 50; i++ {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("broker non pronto su %s", addr)
	return ""
}

// TestMQTTEndToEnd verifica l'intera catena: simulatore -> broker -> ingest -> SQLite.
func TestMQTTEndToEnd(t *testing.T) {
	addr := startBroker(t)

	st, err := store.Open(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	logger := log.New(io.Discard, "", 0)
	h := hub.New()
	ing := ingest.New(st, h, logger)

	sub, err := broker.Start(broker.Config{
		URL:      "tcp://" + addr,
		ClientID: "argine-e2e-sub",
		Topic:    "421342415/#",
	}, ing.Handle)
	if err != nil {
		t.Fatalf("broker.Start: %v", err)
	}
	t.Cleanup(sub.Stop)

	if err := sim.Run(sim.Options{
		Broker:   addr,
		Root:     "421342415",
		Interval: 20 * time.Millisecond,
		Count:    1,
	}, nil); err != nil {
		t.Fatalf("sim.Run: %v", err)
	}

	// Attende che le misure arrivino nel database.
	deadline := time.Now().Add(5 * time.Second)
	var devices []store.Device
	for time.Now().Before(deadline) {
		devices, _ = st.ListDevices()
		if len(devices) >= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(devices) < 4 {
		t.Fatalf("dispositivi ricevuti = %d, attesi almeno 4", len(devices))
	}

	st2, err := st.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st2.Readings < 4 {
		t.Errorf("misure = %d, attese almeno 4", st2.Readings)
	}
}
