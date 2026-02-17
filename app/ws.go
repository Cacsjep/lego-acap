package main

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
)

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type WSHub struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
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

func (h *WSHub) Broadcast(msgType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

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
