package topology

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	data := []byte(`{
		"links": [
			{"from": "Rio Molini 0", "to": "Rio Piazzo 0"},
			{"from": "Rio Val Morela 0", "to": "Rio Piazzo 0"}
		]
	}`)

	topo, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(topo.Links) != 2 {
		t.Fatalf("link inattesi: %+v", topo.Links)
	}
	if topo.Links[0].From != "Rio Molini 0" || topo.Links[0].To != "Rio Piazzo 0" {
		t.Fatalf("primo link inatteso: %+v", topo.Links[0])
	}
}

func TestParseEmpty(t *testing.T) {
	topo, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if len(topo.Links) != 0 {
		t.Fatalf("atteso vuoto, ottenuto: %+v", topo)
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	// Un eventuale campo "nodes" di vecchie configurazioni viene ignorato.
	topo, err := Parse([]byte(`{"nodes": [{"id": "x"}], "links": [{"from": "A", "to": "B"}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(topo.Links) != 1 {
		t.Fatalf("link inattesi: %+v", topo.Links)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("{")); err == nil {
		t.Fatal("atteso errore su JSON non valido")
	}
}

func TestLoadFallback(t *testing.T) {
	fallback := []byte(`{"links": [{"from": "A", "to": "B"}]}`)
	topo, err := Load("davvero-non-esiste.json", fallback)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(topo.Links) != 1 {
		t.Fatalf("atteso il fallback incorporato, ottenuto: %+v", topo)
	}
}

func TestLoadOrCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sottocartella", "topology.json")
	fallback := []byte(`{"links": [{"from": "A", "to": "B"}]}`)

	topo, created, err := LoadOrCreate(path, fallback)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if !created {
		t.Fatal("atteso created=true al primo avvio")
	}
	if len(topo.Links) != 1 {
		t.Fatalf("link inattesi: %+v", topo.Links)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file non creato: %v", err)
	}

	if err := Save(path, Topology{Links: []Link{{From: "C", To: "D"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	topo, created, err = LoadOrCreate(path, fallback)
	if err != nil {
		t.Fatalf("LoadOrCreate (2): %v", err)
	}
	if created {
		t.Error("atteso created=false su file esistente")
	}
	if len(topo.Links) != 1 || topo.Links[0].From != "C" {
		t.Fatalf("il file esistente deve avere precedenza: %+v", topo.Links)
	}
}

func TestLoadOrCreateInvalidFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topology.json")
	if _, _, err := LoadOrCreate(path, []byte("{")); err == nil {
		t.Fatal("atteso errore con fallback non valido")
	}
}
