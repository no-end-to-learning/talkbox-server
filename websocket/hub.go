package websocket

import (
	"encoding/json"
	"sync"
)

type Hub struct {
	clients    map[string]*Client
	userConns  map[string]map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Message struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type ClientMessage struct {
	Action         string          `json:"action"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Type           string          `json:"type,omitempty"`
	Content        json.RawMessage `json:"content,omitempty"`
	ReplyToID      string          `json:"reply_to_id,omitempty"`
}

var HubInstance *Hub

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		userConns:  make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			if h.userConns[client.UserID] == nil {
				h.userConns[client.UserID] = make(map[*Client]bool)
			}
			h.userConns[client.UserID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				if h.userConns[client.UserID] != nil {
					delete(h.userConns[client.UserID], client)
					if len(h.userConns[client.UserID]) == 0 {
						delete(h.userConns, client.UserID)
					}
				}
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) SendToUser(userID string, msg *Message) {
	h.mu.RLock()
	clients := h.userConns[userID]
	h.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			h.unregister <- client
		}
	}
}

func (h *Hub) SendToUsers(userIDs []string, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	for _, userID := range userIDs {
		clients := h.userConns[userID]
		for client := range clients {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.userConns[userID]) > 0
}

func InitHub() {
	HubInstance = NewHub()
	go HubInstance.Run()
}
