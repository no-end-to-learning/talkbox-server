package models

import "time"

type Conversation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // private, group
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationMember struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	Role           string    `json:"role"` // owner, admin, member
	Nickname       string    `json:"nickname"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ConversationResponse struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Name      string           `json:"name"`
	Avatar    string           `json:"avatar"`
	OwnerID   string           `json:"owner_id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Members   []MemberWithUser `json:"members,omitempty"`
	Bots      []BotResponse    `json:"bots,omitempty"`
}

type MemberWithUser struct {
	ID       string       `json:"id"`
	UserID   string       `json:"user_id"`
	Role     string       `json:"role"`
	Nickname string       `json:"nickname"`
	User     UserResponse `json:"user"`
}

func (c *Conversation) ToResponse() *ConversationResponse {
	return &ConversationResponse{
		ID:        c.ID,
		Type:      c.Type,
		Name:      c.Name,
		Avatar:    c.Avatar,
		OwnerID:   c.OwnerID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
