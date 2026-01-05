package handlers

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"talkbox/database"
	"talkbox/middleware"
	"talkbox/models"
	"talkbox/utils"
)

type FriendRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func GetFriends(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT f.id, f.user_id, f.friend_id, f.status, f.created_at, f.updated_at,
			   u.id, u.username, u.nickname, u.avatar
		FROM friendships f
		JOIN users u ON u.id = f.friend_id
		WHERE f.user_id = ? AND f.status = 'accepted'
		ORDER BY u.nickname
	`, userID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var friends []models.FriendWithUser
	for rows.Next() {
		var f models.FriendWithUser
		var user models.User
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt, &f.UpdatedAt,
			&user.ID, &user.Username, &user.Nickname, &user.Avatar,
		); err != nil {
			continue
		}
		f.Friend = *user.ToResponse()
		friends = append(friends, f)
	}

	if friends == nil {
		friends = []models.FriendWithUser{}
	}

	utils.Success(c, friends)
}

func GetFriendRequests(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT f.id, f.user_id, f.friend_id, f.status, f.created_at, f.updated_at,
			   u.id, u.username, u.nickname, u.avatar
		FROM friendships f
		JOIN users u ON u.id = f.user_id
		WHERE f.friend_id = ? AND f.status = 'pending'
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var requests []models.FriendWithUser
	for rows.Next() {
		var f models.FriendWithUser
		var user models.User
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt, &f.UpdatedAt,
			&user.ID, &user.Username, &user.Nickname, &user.Avatar,
		); err != nil {
			continue
		}
		f.Friend = *user.ToResponse()
		requests = append(requests, f)
	}

	if requests == nil {
		requests = []models.FriendWithUser{}
	}

	utils.Success(c, requests)
}

func SendFriendRequest(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req FriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if req.UserID == userID {
		utils.BadRequest(c, "cannot add yourself as friend")
		return
	}

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", req.UserID).Scan(&exists)
	if err != nil || !exists {
		utils.NotFound(c, "user not found")
		return
	}

	var status string
	err = database.DB.QueryRow(
		"SELECT status FROM friendships WHERE user_id = ? AND friend_id = ?",
		userID, req.UserID,
	).Scan(&status)

	if err == nil {
		if status == "accepted" {
			utils.BadRequest(c, "already friends")
		} else if status == "pending" {
			utils.BadRequest(c, "friend request already sent")
		} else {
			utils.BadRequest(c, "cannot send request")
		}
		return
	}

	err = database.DB.QueryRow(
		"SELECT status FROM friendships WHERE user_id = ? AND friend_id = ?",
		req.UserID, userID,
	).Scan(&status)

	if err == nil && status == "pending" {
		AcceptFriendRequestInternal(c, userID, req.UserID)
		return
	}

	id := utils.GenerateUUID()
	now := time.Now()

	_, err = database.DB.Exec(
		"INSERT INTO friendships (id, user_id, friend_id, status, created_at, updated_at) VALUES (?, ?, ?, 'pending', ?, ?)",
		id, userID, req.UserID, now, now,
	)
	if err != nil {
		utils.InternalError(c, "failed to send friend request")
		return
	}

	utils.Success(c, gin.H{"message": "friend request sent"})
}

func AcceptFriendRequest(c *gin.Context) {
	userID := middleware.GetUserID(c)
	friendID := c.Param("user_id")

	AcceptFriendRequestInternal(c, userID, friendID)
}

func AcceptFriendRequestInternal(c *gin.Context, userID, friendID string) {
	tx, err := database.DB.Begin()
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	now := time.Now()

	result, err := tx.Exec(
		"UPDATE friendships SET status = 'accepted', updated_at = ? WHERE user_id = ? AND friend_id = ? AND status = 'pending'",
		now, friendID, userID,
	)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to accept request")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "database error")
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		utils.NotFound(c, "friend request not found")
		return
	}

	id := utils.GenerateUUID()
	_, err = tx.Exec(
		"INSERT INTO friendships (id, user_id, friend_id, status, created_at, updated_at) VALUES (?, ?, ?, 'accepted', ?, ?)",
		id, userID, friendID, now, now,
	)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to create friendship")
		return
	}

	if err := tx.Commit(); err != nil {
		utils.InternalError(c, "failed to commit transaction")
		return
	}

	utils.Success(c, gin.H{"message": "friend request accepted"})
}

func DeleteFriend(c *gin.Context) {
	userID := middleware.GetUserID(c)
	friendID := c.Param("user_id")

	tx, err := database.DB.Begin()
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	_, err = tx.Exec(
		"DELETE FROM friendships WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)",
		userID, friendID, friendID, userID,
	)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete friend")
		return
	}

	if err := tx.Commit(); err != nil {
		utils.InternalError(c, "failed to commit transaction")
		return
	}

	utils.Success(c, nil)
}

func FindOrCreatePrivateConversation(userA, userB string) (string, error) {
	if userA > userB {
		userA, userB = userB, userA
	}

	var convID string
	err := database.DB.QueryRow(`
		SELECT c.id FROM conversations c
		JOIN conversation_members m1 ON c.id = m1.conversation_id AND m1.user_id = ?
		JOIN conversation_members m2 ON c.id = m2.conversation_id AND m2.user_id = ?
		WHERE c.type = 'private'
	`, userA, userB).Scan(&convID)

	if err == nil {
		return convID, nil
	}

	if err != sql.ErrNoRows {
		return "", err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}

	convID = utils.GenerateUUID()
	now := time.Now()

	_, err = tx.Exec(
		"INSERT INTO conversations (id, type, created_at, updated_at) VALUES (?, 'private', ?, ?)",
		convID, now, now,
	)
	if err != nil {
		tx.Rollback()
		return "", err
	}

	for _, uid := range []string{userA, userB} {
		memberID := utils.GenerateUUID()
		_, err = tx.Exec(
			"INSERT INTO conversation_members (id, conversation_id, user_id, role, created_at, updated_at) VALUES (?, ?, ?, 'member', ?, ?)",
			memberID, convID, uid, now, now,
		)
		if err != nil {
			tx.Rollback()
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return convID, nil
}
