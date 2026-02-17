package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"mini-discord/internal/storage"

	gws "github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = gws.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Message struct {
	Type    string   `json:"type"`
	Author  string   `json:"author,omitempty"`
	Channel string   `json:"channel,omitempty"`
	Text    string   `json:"text,omitempty"`
	Users   []string `json:"users,omitempty"`
}

type Client struct {
	hub  *Hub
	conn *gws.Conn
	send chan Message
	name string
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	store      *storage.Store
}

func NewHub(store *storage.Store) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		store:      store,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.broadcastPresence()
			log.Printf("[chat] client connected, total: %d", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.broadcastPresence()
				log.Printf("[chat] client disconnected, total: %d", len(h.clients))
			}

		case msg := <-h.broadcast:
			if msg.Type == "" {
				msg.Type = "chat"
			}
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) broadcastPresence() {
	users := make([]string, 0, len(h.clients))
	for c := range h.clients {
		if c.name != "" {
			users = append(users, c.name)
		}
	}

	msg := Message{Type: "presence", Users: users}
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[chat] websocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan Message, 256),
	}
	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if gws.IsUnexpectedCloseError(err, gws.CloseGoingAway, gws.CloseAbnormalClosure) {
				log.Printf("[chat] read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		msg.Author = strings.TrimSpace(msg.Author)
		msg.Text = strings.TrimSpace(msg.Text)
		msg.Channel = strings.TrimSpace(msg.Channel)
		if msg.Channel == "" {
			msg.Channel = "general"
		}
		if msg.Author == "" || msg.Text == "" {
			continue
		}

		c.name = msg.Author
		if c.hub.store != nil {
			if err := c.hub.store.SaveMessage(msg.Author, msg.Channel, msg.Text); err != nil {
				log.Printf("[chat] save message error: %v", err)
			}
		}
		c.hub.broadcast <- Message{
			Type:    "chat",
			Author:  msg.Author,
			Text:    msg.Text,
			Channel: msg.Channel,
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gws.CloseMessage, []byte{})
				return
			}

			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := c.conn.WriteMessage(gws.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
