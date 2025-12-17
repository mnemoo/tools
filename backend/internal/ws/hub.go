// Package ws provides WebSocket functionality for real-time communication.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// MessageType defines the type of WebSocket message.
type MessageType string

const (
	// Loading progress messages
	MsgLoadingStarted  MessageType = "loading_started"
	MsgLoadingProgress MessageType = "loading_progress"
	MsgLoadingComplete MessageType = "loading_complete"
	MsgLoadingError    MessageType = "loading_error"
	MsgReloadStarted   MessageType = "reload_started"

	// Priority control messages
	MsgPriorityChanged MessageType = "priority_changed"

	// LGS session messages
	MsgLGSSessionUpdate  MessageType = "lgs_session_update"
	MsgLGSSessionsUpdate MessageType = "lgs_sessions_update"
)

// Message represents a WebSocket message sent to clients.
type Message struct {
	Type    MessageType `json:"type"`
	Mode    string      `json:"mode,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

// LoadingProgress contains progress information for book loading.
type LoadingProgress struct {
	Mode           string  `json:"mode"`
	EventsFile     string  `json:"events_file"`
	CurrentLine    int     `json:"current_line"`
	TotalLines     int     `json:"total_lines,omitempty"` // May be 0 if unknown
	BytesRead      int64   `json:"bytes_read"`
	TotalBytes     int64   `json:"total_bytes"`
	PercentBytes   float64 `json:"percent_bytes"`
	PercentLines   float64 `json:"percent_lines,omitempty"`
	Priority       string  `json:"priority"` // "low", "high"
	ElapsedMs      int64   `json:"elapsed_ms"`
	EstimatedMs    int64   `json:"estimated_ms,omitempty"`
	LinesPerSecond float64 `json:"lines_per_second"`
}

// Client represents a connected WebSocket client.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected, total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected, total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, disconnect
					h.mu.RUnlock()
					h.mu.Lock()
					close(client.send)
					delete(h.clients, client)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling WebSocket message: %v", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		log.Printf("Broadcast channel full, message dropped")
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// ServeWs handles WebSocket requests from clients.
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
		// We don't process incoming messages for now, just keep connection alive
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}
