package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"talkbox/database"
	"talkbox/models"
	"talkbox/utils"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	ID     string
	UserID string
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Action {
	case "ping":
		c.sendPong()
	case "send_message":
		c.handleSendMessage(&msg)
	}
}

func (c *Client) sendPong() {
	response := &Message{Event: "pong"}
	data, _ := json.Marshal(response)
	c.Send <- data
}

func (c *Client) handleSendMessage(msg *ClientMessage) {
	if !c.isConversationMember(msg.ConversationID) {
		return
	}

	msgID := uuid.New().String()
	now := time.Now()

	_, err := database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, sender_type, type, content, reply_to_id, created_at, updated_at)
		VALUES (?, ?, ?, 'user', ?, ?, ?, ?, ?)
	`, msgID, msg.ConversationID, c.UserID, msg.Type, string(msg.Content), msg.ReplyToID, now, now)

	if err != nil {
		return
	}

	database.DB.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", now, msg.ConversationID)

	var user models.User
	database.DB.QueryRow(
		"SELECT id, username, nickname, avatar FROM users WHERE id = ?",
		c.UserID,
	).Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar)

	broadcastMsg := &Message{
		Event: "new_message",
		Data: map[string]interface{}{
			"id":              msgID,
			"conversation_id": msg.ConversationID,
			"sender": map[string]interface{}{
				"id":       c.UserID,
				"type":     "user",
				"nickname": user.Nickname,
				"avatar":   user.Avatar,
			},
			"type":       msg.Type,
			"content":    msg.Content,
			"reply_to":   nil,
			"created_at": now.Format(time.RFC3339),
		},
	}

	memberIDs := c.getConversationMembers(msg.ConversationID)
	c.Hub.SendToUsers(memberIDs, broadcastMsg)

	if msg.Type == "text" {
		var textContent models.TextContent
		if err := json.Unmarshal(msg.Content, &textContent); err == nil && len(textContent.Mentions) > 0 {
			for _, mentionedUserID := range textContent.Mentions {
				mentionID := uuid.New().String()
				database.DB.Exec(
					"INSERT INTO mentions (id, message_id, user_id, created_at) VALUES (?, ?, ?, ?)",
					mentionID, msgID, mentionedUserID, now,
				)

				c.Hub.SendToUser(mentionedUserID, &Message{
					Event: "mentioned",
					Data: map[string]interface{}{
						"message_id":      msgID,
						"conversation_id": msg.ConversationID,
						"sender_name":     user.Nickname,
					},
				})
			}
		}
	}
}

func (c *Client) isConversationMember(convID string) bool {
	var exists bool
	database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = ? AND user_id = ?)",
		convID, c.UserID,
	).Scan(&exists)
	return exists
}

func (c *Client) getConversationMembers(convID string) []string {
	rows, err := database.DB.Query(
		"SELECT user_id FROM conversation_members WHERE conversation_id = ?",
		convID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err == nil {
			members = append(members, userID)
		}
	}
	return members
}

func HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:     uuid.New().String(),
		UserID: claims.UserID,
		Hub:    HubInstance,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	client.Hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}

func BroadcastToConversation(convID string, msg *Message) {
	rows, err := database.DB.Query(
		"SELECT user_id FROM conversation_members WHERE conversation_id = ?",
		convID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err == nil {
			members = append(members, userID)
		}
	}

	HubInstance.SendToUsers(members, msg)
}
