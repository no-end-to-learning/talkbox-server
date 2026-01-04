package models

import "time"

type Bot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Avatar      string    `json:"avatar"`
	Description string    `json:"description"`
	Token       string    `json:"-"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BotResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Avatar      string    `json:"avatar"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type BotWithToken struct {
	Bot
	Token string `json:"token"`
}

type BotConversation struct {
	ID             string    `json:"id"`
	BotID          string    `json:"bot_id"`
	ConversationID string    `json:"conversation_id"`
	AddedBy        string    `json:"added_by"`
	CreatedAt      time.Time `json:"created_at"`
}

func (b *Bot) ToResponse() *BotResponse {
	return &BotResponse{
		ID:          b.ID,
		Name:        b.Name,
		Avatar:      b.Avatar,
		Description: b.Description,
		CreatedAt:   b.CreatedAt,
	}
}
