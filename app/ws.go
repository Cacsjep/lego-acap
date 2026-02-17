package main

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
)

// WebSocket message type constants shared with the frontend.
const (
	MsgDownloadProgress = "download_progress"
	MsgDownloadComplete = "download_complete"
	MsgDownloadError    = "download_error"
	MsgLegoOutput       = "lego_output"
	MsgLegoComplete     = "lego_complete"
	MsgLegoError        = "lego_error"
)

// WSMessage is the envelope sent to WebSocket clients.
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// WSHub broadcasts messages to all connected WebSocket clients.
type WSHub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*websocket.Conn]bool),
	}
}

func (h *WSHub) Register(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *WSHub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

func (h *WSHub) Broadcast(msgType string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := WSMessage{Type: msgType, Data: data}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for client := range h.clients {
		if err := client.WriteMessage(websocket.TextMessage, payload); err != nil {
			client.Close()
			delete(h.clients, client)
		}
	}
}
