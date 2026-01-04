package models

import "time"

type Friendship struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	FriendID  string    `json:"friend_id"`
	Status    string    `json:"status"` // pending, accepted, blocked
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FriendWithUser struct {
	Friendship
	Friend UserResponse `json:"friend"`
}
