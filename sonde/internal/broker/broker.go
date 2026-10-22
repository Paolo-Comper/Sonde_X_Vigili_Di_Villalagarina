// Package broker si collega al broker MQTT e consegna a un handler ogni
// messaggio di un filtro di topic.
package broker

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config contiene i parametri di connessione MQTT.
type Config struct {
	URL      string // es. tcp://broker.hivemq.com:1883
	ClientID string
	Topic    string // filtro di sottoscrizione, es. 421342415/#
	Username string
	Password string
}

// Handler viene chiamato per ogni messaggio ricevuto.
type Handler func(topic string, payload []byte)

// Broker incapsula un client MQTT paho.
type Broker struct {
	client mqtt.Client
	topic  string
}

// Start si collega al broker e sottoscrive cfg.Topic. Ritorna quando la
// sottoscrizione e' attiva; le riconnessioni sono gestite da paho.
func Start(cfg Config, handler Handler) (*Broker, error) {
	url := cfg.URL
	if !strings.Contains(url, "://") {
		url = "tcp://" + url
	}
	opts := mqtt.NewClientOptions().
		AddBroker(url).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(5 * time.Second).
		SetConnectTimeout(10 * time.Second)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		token := c.Subscribe(cfg.Topic, 0, func(_ mqtt.Client, m mqtt.Message) {
			handler(m.Topic(), m.Payload())
		})
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("MQTT subscribe error: %v", err)
		} else {
			log.Printf("MQTT subscribed to %q", cfg.Topic)
		}
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(12 * time.Second) {
		client.Disconnect(0)
		return nil, fmt.Errorf("MQTT connect: timeout")
	}
	if err := token.Error(); err != nil {
		client.Disconnect(0)
		return nil, fmt.Errorf("MQTT connect: %w", err)
	}
	if !client.IsConnected() {
		client.Disconnect(0)
		return nil, fmt.Errorf("MQTT connect: non connesso")
	}

	return &Broker{client: client, topic: cfg.Topic}, nil
}

// Stop si disconnette dal broker.
func (b *Broker) Stop() {
	if b == nil || b.client == nil {
		return
	}
	b.client.Unsubscribe(b.topic)
	b.client.Disconnect(250)
}

// Connected indica se il client e' attualmente connesso.
func (b *Broker) Connected() bool {
	return b != nil && b.client != nil && b.client.IsConnected()
}

// Status descrive lo stato della connessione MQTT.
type Status struct {
	Connected bool   `json:"connected"`
	Broker    string `json:"broker"`
	Topic     string `json:"topic"`
	Error     string `json:"error,omitempty"`
}

// Manager mantiene una sola connessione MQTT e la riconfigura a caldo.
type Manager struct {
	mu      sync.Mutex
	handler Handler
	log     *log.Logger
	client  *Broker
	cfg     Config
	lastErr error
}

// NewManager crea un manager che consegna i messaggi a handler.
func NewManager(handler Handler, logger *log.Logger) *Manager {
	return &Manager{handler: handler, log: logger}
}

// Apply chiude la connessione precedente e ne apre una nuova.
func (m *Manager) Apply(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.ClientID == "" {
		cfg.ClientID = m.cfg.ClientID
		if cfg.ClientID == "" {
			cfg.ClientID = fmt.Sprintf("sonde-%d", time.Now().UnixNano())
		}
	}
	if cfg.URL == "" {
		cfg.URL = m.cfg.URL
	}
	m.cfg = cfg
	if m.client != nil {
		m.client.Stop()
		m.client = nil
	}

	b, err := Start(cfg, m.handler)
	if err != nil {
		m.lastErr = err
		if m.log != nil {
			m.log.Printf("MQTT non connesso (%v)", err)
		}
		return err
	}
	m.client = b
	m.lastErr = nil
	return nil
}

// Stop chiude la connessione.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		m.client.Stop()
		m.client = nil
	}
}

// Status restituisce lo stato corrente.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{Broker: m.cfg.URL, Topic: m.cfg.Topic}
	if m.client != nil {
		st.Connected = m.client.Connected()
	}
	if m.lastErr != nil {
		st.Error = m.lastErr.Error()
	}
	return st
}
