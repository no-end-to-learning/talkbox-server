package models

import (
	"encoding/json"
	"time"
)

type Message struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	SenderID       string          `json:"sender_id"`
	SenderType     string          `json:"sender_type"` // user, bot
	Type           string          `json:"type"`        // text, image, video, file, card
	Content        json.RawMessage `json:"content"`
	ReplyToID      *string         `json:"reply_to_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type MessageResponse struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	Sender         SenderInfo      `json:"sender"`
	Type           string          `json:"type"`
	Content        json.RawMessage `json:"content"`
	ReplyToID      string          `json:"reply_to_id,omitempty"`
	ReplyTo        *ReplyInfo      `json:"reply_to,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type SenderInfo struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // user, bot
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type ReplyInfo struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Content    json.RawMessage `json:"content"`
	SenderName string          `json:"sender_name"`
}

// Content types
type TextContent struct {
	Text     string   `json:"text"`
	Mentions []string `json:"mentions,omitempty"`
}

type ImageContent struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Size      int64  `json:"size"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

type VideoContent struct {
	URL       string `json:"url"`
	Duration  int    `json:"duration"`
	Size      int64  `json:"size"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

type FileContent struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

type CardContent struct {
	Color   string `json:"color,omitempty"` // default #1890FF
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Note    string `json:"note,omitempty"`
	URL     string `json:"url,omitempty"`
}

type Mention struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
