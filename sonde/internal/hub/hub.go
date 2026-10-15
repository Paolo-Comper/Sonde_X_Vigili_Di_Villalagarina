// Package hub implementa un piccolo fan-out usato per inviare gli aggiornamenti
// live alla dashboard via Server-Sent Events.
package hub

import "sync"

// Event e' un messaggio con nome consegnato a ogni sottoscrittore SSE.
type Event struct {
	Type string
	Data []byte
}

// Hub distribuisce gli eventi ai sottoscrittori. Un sottoscrittore lento perde
// eventi invece di bloccare chi pubblica.
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

// New crea un hub vuoto.
func New() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
}

// Subscribe registra un nuovo sottoscrittore; va annullato con Unsubscribe.
func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe rimuove un sottoscrittore e chiude il suo canale.
func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Broadcast invia un evento a tutti i sottoscrittori senza bloccarsi.
func (h *Hub) Broadcast(typ string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- Event{Type: typ, Data: data}:
		default:
			// Il sottoscrittore non tiene il passo: evento scartato.
		}
	}
}

// Clients riporta il numero di sottoscrittori connessi.
func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
