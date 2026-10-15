// Package broker si collega al broker MQTT e consegna a un handler ogni
// messaggio di un filtro di topic.
package broker

import (
	"fmt"
	"log"
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
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.URL).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
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
	if token.WaitTimeout(15*time.Second) && token.Error() != nil {
		return nil, fmt.Errorf("MQTT connect: %w", token.Error())
	}
	if !client.IsConnected() && !client.IsConnectionOpen() {
		return nil, fmt.Errorf("MQTT connect: timeout")
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
