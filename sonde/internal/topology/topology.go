// Package topology descrive come le sonde sono collegate tra loro (affluenze).
// from/to si riferiscono a sonde reali (per id o label): i collegamenti verso
// punti senza sonda vengono semplicemente ignorati, cosi' non si disegnano
// frecce che puntano nel vuoto.
package topology

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Link e' un collegamento diretto da una sonda a valle (affluenza).
type Link struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Topology e' l'insieme dei collegamenti configurati.
type Topology struct {
	Links []Link `json:"links"`
}

// Load legge il file indicato. Se il file non esiste usa fallback (di norma il
// default incorporato nel binario), senza scrivere nulla.
func Load(path string, fallback []byte) (Topology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Topology{}, err
		}
		data = fallback
	}
	return Parse(data)
}

// LoadOrCreate legge il file e, se manca, lo crea usando fallback.
// Il secondo valore indica se il file e' stato creato.
func LoadOrCreate(path string, fallback []byte) (Topology, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		t, err := Parse(data)
		return t, false, err
	}
	if !os.IsNotExist(err) {
		return Topology{}, false, err
	}

	t, err := Parse(fallback)
	if err != nil {
		return Topology{}, false, err
	}
	if err := Save(path, t); err != nil {
		return Topology{}, false, fmt.Errorf("topology: crea %s: %w", path, err)
	}
	return t, true, nil
}

// Save scrive la topologia in modo atomico, creando la cartella se serve.
func Save(path string, t Topology) error {
	if t.Links == nil {
		t.Links = []Link{}
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".topology-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Parse interpreta il JSON della topologia.
func Parse(data []byte) (Topology, error) {
	t := Topology{}
	if len(data) == 0 {
		return t, nil
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return Topology{}, fmt.Errorf("topology: %w", err)
	}
	if t.Links == nil {
		t.Links = []Link{}
	}
	return t, nil
}
