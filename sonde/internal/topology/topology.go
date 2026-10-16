// Package topology descrive come le sonde sono collegate tra loro (affluenze).
// from/to possono riferirsi a un device (per id o label) oppure a un nodo
// virtuale definito in "nodes" (es. un fiume senza sonda).
package topology

import (
	"encoding/json"
	"fmt"
	"os"
)

// Node e' un punto noto della rete che non coincide necessariamente con una
// sonda (ad esempio lo sbocco in un fiume).
type Node struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

// Link e' un collegamento diretto da una sonda a valle (affluenza).
type Link struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Topology e' l'insieme di nodi virtuali e collegamenti configurati.
type Topology struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// Load legge il file indicato. Se il file non esiste usa fallback (di norma il
// default incorporato nel binario), senza segnalare errore.
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

// Parse interpreta il JSON della topologia.
func Parse(data []byte) (Topology, error) {
	t := Topology{}
	if len(data) == 0 {
		return t, nil
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return Topology{}, fmt.Errorf("topology: %w", err)
	}
	if t.Nodes == nil {
		t.Nodes = []Node{}
	}
	if t.Links == nil {
		t.Links = []Link{}
	}
	return t, nil
}
