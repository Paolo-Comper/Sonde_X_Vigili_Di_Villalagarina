package topology

import (
	"testing"
)

func TestParse(t *testing.T) {
	data := []byte(`{
		"nodes": [{"id": "Fiume Adige", "label": "Fiume Adige", "lat": 45.886, "lon": 11.044}],
		"links": [{"from": "Rio Molini 0", "to": "Rio Piazzo 0"}]
	}`)

	topo, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(topo.Nodes) != 1 || topo.Nodes[0].ID != "Fiume Adige" {
		t.Fatalf("nodi inattesi: %+v", topo.Nodes)
	}
	if len(topo.Links) != 1 || topo.Links[0].From != "Rio Molini 0" || topo.Links[0].To != "Rio Piazzo 0" {
		t.Fatalf("link inattesi: %+v", topo.Links)
	}
}

func TestParseEmpty(t *testing.T) {
	topo, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if len(topo.Nodes) != 0 || len(topo.Links) != 0 {
		t.Fatalf("atteso vuoto, ottenuto: %+v", topo)
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
