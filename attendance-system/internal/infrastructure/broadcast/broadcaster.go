package broadcast

import (
	"encoding/json"
	"sync"
)

type Broadcaster struct {
	mu      sync.Mutex
	clients map[chan []byte]bool
}

var Global = &Broadcaster{
	clients: make(map[chan []byte]bool),
}

func (b *Broadcaster) AddClient(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[ch] = true
}

func (b *Broadcaster) RemoveClient(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *Broadcaster) Broadcast(eventType string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()

	payload, err := json.Marshal(map[string]any{
		"type": eventType,
		"data": data,
	})
	if err != nil {
		return
	}

	for ch := range b.clients {
		select {
		case ch <- payload:
		default:
			// client channel full, skip to avoid blocking the main server threads
		}
	}
}
