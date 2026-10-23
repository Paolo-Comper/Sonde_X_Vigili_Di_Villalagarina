package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTopicFilter(t *testing.T) {
	cases := []struct{ in, want string }{
		{"421342415", "421342415/#"},
		{"421342415/", "421342415/#"},
		{"421342415/#", "421342415/#"},
		{"a/+/data", "a/+/data"},
		{"#", "#"},
		{"", "#"},
	}
	for _, c := range cases {
		if got := TopicFilter(c.in); got != c.want {
			t.Errorf("TopicFilter(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrefixFromTopic(t *testing.T) {
	cases := []struct{ in, want string }{
		{"421342415/#", "421342415"},
		{"421342415/+/data", "421342415"},
		{"421342415/", "421342415"},
		{"421342415", "421342415"},
	}
	for _, c := range cases {
		if got := PrefixFromTopic(c.in); got != c.want {
			t.Errorf("PrefixFromTopic(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizedAndHelpers(t *testing.T) {
	m := MQTT{Broker: " broker:1883 ", Prefix: " 421342415/ "}.Normalized()
	if m.Broker != "broker:1883" || m.Prefix != "421342415" || m.Topic != "421342415/#" {
		t.Fatalf("normalized = %+v", m)
	}
	if m.DeviceTopic("Rio_Molini_0") != "421342415/Rio_Molini_0" {
		t.Errorf("DeviceTopic = %q", m.DeviceTopic("Rio_Molini_0"))
	}
	if m.BrokerURL() != "tcp://broker:1883" {
		t.Errorf("BrokerURL = %q", m.BrokerURL())
	}
	wild := MQTT{Prefix: "a/+/data"}.Normalized()
	if wild.Group() != "" || wild.DeviceTopic("x") != "" {
		t.Errorf("con wildcard il gruppo deve essere vuoto: %+v", wild)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, ok, err := Load(path); err != nil || ok {
		t.Fatalf("Load iniziale = ok:%v err:%v", ok, err)
	}
	in := MQTT{Broker: "broker:1883", Prefix: "421342415", Username: "u", Password: "p"}
	if err := in.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := Load(path)
	if err != nil || !ok {
		t.Fatalf("Load = ok:%v err:%v", ok, err)
	}
	if got.Topic != "421342415/#" || got.Password != "p" || got.Username != "u" {
		t.Fatalf("Load = %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("permessi troppo larghi: %v", info.Mode().Perm())
	}
}
