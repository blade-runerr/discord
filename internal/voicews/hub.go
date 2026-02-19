package voicews

import (
	"encoding/json"
	"log"
)

type envelope struct {
	Type      string   `json:"type"`
	ID        string   `json:"id,omitempty"`
	From      string   `json:"from,omitempty"`
	To        string   `json:"to,omitempty"`
	Peers     []string `json:"peers,omitempty"`
	SDP       string   `json:"sdp,omitempty"`
	Candidate string   `json:"candidate,omitempty"`
}

type routedSignal struct {
	from string
	to   string
	msg  envelope
}

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	signals    chan routedSignal
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		signals:    make(chan routedSignal, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			existingPeers := make([]string, 0, len(h.clients))
			for id := range h.clients {
				existingPeers = append(existingPeers, id)
			}

			h.clients[client.id] = client
			log.Printf("[voice] client connected: %s total=%d", client.id, len(h.clients))

			client.sendJSON(envelope{
				Type:  "welcome",
				ID:    client.id,
				Peers: existingPeers,
			})

			h.broadcast(envelope{
				Type: "peer_joined",
				ID:   client.id,
			}, client.id)

		case client := <-h.unregister:
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.send)
				log.Printf("[voice] client disconnected: %s total=%d", client.id, len(h.clients))
				h.broadcast(envelope{
					Type: "peer_left",
					ID:   client.id,
				}, "")
			}

		case signal := <-h.signals:
			dst, ok := h.clients[signal.to]
			if !ok {
				continue
			}
			msg := signal.msg
			msg.From = signal.from
			dst.sendJSON(msg)
		}
	}
}

func (h *Hub) broadcast(msg envelope, exceptID string) {
	for id, client := range h.clients {
		if id == exceptID {
			continue
		}
		client.sendJSON(msg)
	}
}

func (c *Client) sendJSON(v envelope) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
	}
}
