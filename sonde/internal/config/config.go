// Package config gestisce le impostazioni MQTT modificabili dalla dashboard.
//
// Le impostazioni (credenziali comprese) stanno in un file JSON separato dal
// database: la console SQL della dashboard puo' interrogare il database, ma
// non deve poter leggere la password del broker.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// MQTT e' la configurazione della connessione al broker.
type MQTT struct {
	Broker   string `json:"broker"`
	Prefix   string `json:"prefix"`
	Topic    string `json:"topic"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// Normalized ripulisce i campi e ricalcola il filtro di sottoscrizione.
func (m MQTT) Normalized() MQTT {
	m.Broker = strings.TrimSpace(m.Broker)
	if m.Broker == "" {
		m.Broker = "broker.hivemq.com:1883"
	}
	m.Prefix = strings.TrimRight(strings.TrimSpace(m.Prefix), "/")
	m.Topic = TopicFilter(m.Prefix)
	return m
}

// TopicFilter trasforma un prefisso in filtro MQTT. Un filtro gia' completo
// (con + o #) viene lasciato com'e'.
func TopicFilter(prefix string) string {
	s := strings.TrimSpace(prefix)
	if s == "" {
		return "#"
	}
	if strings.ContainsAny(s, "+#") {
		return s
	}
	return strings.TrimRight(s, "/") + "/#"
}

// PrefixFromTopic ricava il prefisso da un filtro, es. "a/b/#" -> "a/b".
func PrefixFromTopic(topic string) string {
	s := strings.TrimSpace(topic)
	if i := strings.IndexAny(s, "+#"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(s, "/")
}

// Group e' il gruppo salvato nell'anagrafica: vuoto se il prefisso contiene
// wildcard e quindi non identifica una cartella precisa.
func (m MQTT) Group() string {
	if strings.ContainsAny(m.Prefix, "+#") {
		return ""
	}
	return m.Prefix
}

// DeviceTopic restituisce il topic canonico di un dispositivo.
func (m MQTT) DeviceTopic(id string) string {
	group := m.Group()
	if group == "" {
		return ""
	}
	return group + "/" + id
}

// BrokerURL aggiunge lo schema tcp:// se manca.
func (m MQTT) BrokerURL() string {
	b := strings.TrimSpace(m.Broker)
	if strings.Contains(b, "://") {
		return b
	}
	return "tcp://" + b
}

// Load legge la configurazione dal file. Il secondo valore indica se esiste.
func Load(path string) (MQTT, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return MQTT{}, false, nil
	}
	if err != nil {
		return MQTT{}, false, err
	}
	var m MQTT
	if err := json.Unmarshal(data, &m); err != nil {
		return MQTT{}, false, err
	}
	return m.Normalized(), true, nil
}

// Save scrive la configurazione con permessi riservati al proprietario.
func (m MQTT) Save(path string) error {
	m = m.Normalized()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Su Windows il rename non sovrascrive un file esistente.
		_ = os.Remove(path)
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}
