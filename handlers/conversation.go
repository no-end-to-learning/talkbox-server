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

type CreateConversationRequest struct {
	Name      string   `json:"name" binding:"required"`
	Avatar    string   `json:"avatar"`
	MemberIDs []string `json:"member_ids"`
}

type UpdateConversationRequest struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type AddMembersRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}

type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required,oneof=admin member"`
}

func GetConversations(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT c.id, c.type, c.name, c.avatar, c.owner_id, c.created_at, c.updated_at
		FROM conversations c
		JOIN conversation_members m ON c.id = m.conversation_id
		WHERE m.user_id = ?
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var conversations []models.ConversationResponse
	for rows.Next() {
		var conv models.Conversation
		if err := rows.Scan(&conv.ID, &conv.Type, &conv.Name, &conv.Avatar, &conv.OwnerID, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			continue
		}
		conversations = append(conversations, *conv.ToResponse())
	}

	if conversations == nil {
		conversations = []models.ConversationResponse{}
	}

	utils.Success(c, conversations)
}

func CreateConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	convID := utils.GenerateUUID()
	now := time.Now()

	_, err = tx.Exec(
		"INSERT INTO conversations (id, type, name, avatar, owner_id, created_at, updated_at) VALUES (?, 'group', ?, ?, ?, ?, ?)",
		convID, req.Name, req.Avatar, userID, now, now,
	)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to create conversation")
		return
	}

	memberID := utils.GenerateUUID()
	_, err = tx.Exec(
		"INSERT INTO conversation_members (id, conversation_id, user_id, role, created_at, updated_at) VALUES (?, ?, ?, 'owner', ?, ?)",
		memberID, convID, userID, now, now,
	)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to add owner")
		return
	}

	for _, uid := range req.MemberIDs {
		if uid == userID {
			continue
		}
		mid := utils.GenerateUUID()
		_, err = tx.Exec(
			"INSERT INTO conversation_members (id, conversation_id, user_id, role, created_at, updated_at) VALUES (?, ?, ?, 'member', ?, ?)",
			mid, convID, uid, now, now,
		)
		if err != nil {
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		utils.InternalError(c, "failed to commit transaction")
		return
	}

	utils.Success(c, gin.H{"id": convID})
}

func GetConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	if !isConversationMember(convID, userID) {
		utils.Forbidden(c, "not a member of this conversation")
		return
	}

	var conv models.Conversation
	err := database.DB.QueryRow(
		"SELECT id, type, name, avatar, owner_id, created_at, updated_at FROM conversations WHERE id = ?",
		convID,
	).Scan(&conv.ID, &conv.Type, &conv.Name, &conv.Avatar, &conv.OwnerID, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		utils.NotFound(c, "conversation not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	rows, err := database.DB.Query(`
		SELECT m.id, m.user_id, m.role, m.nickname, u.username, u.nickname as user_nickname, u.avatar
		FROM conversation_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.conversation_id = ?
	`, convID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var members []models.MemberWithUser
	for rows.Next() {
		var m models.MemberWithUser
		var user models.User
		if err := rows.Scan(&m.ID, &m.UserID, &m.Role, &m.Nickname, &user.Username, &user.Nickname, &user.Avatar); err != nil {
			continue
		}
		user.ID = m.UserID
		m.User = *user.ToResponse()
		members = append(members, m)
	}

	resp := conv.ToResponse()
	resp.Members = members

	utils.Success(c, resp)
}

func UpdateConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	role := getConversationRole(convID, userID)
	if role != "owner" && role != "admin" {
		utils.Forbidden(c, "only owner or admin can update conversation")
		return
	}

	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	_, err := database.DB.Exec(
		"UPDATE conversations SET name = COALESCE(NULLIF(?, ''), name), avatar = COALESCE(NULLIF(?, ''), avatar), updated_at = ? WHERE id = ?",
		req.Name, req.Avatar, time.Now(), convID,
	)
	if err != nil {
		utils.InternalError(c, "failed to update conversation")
		return
	}

	GetConversation(c)
}

func DeleteConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	role := getConversationRole(convID, userID)
	if role != "owner" {
		utils.Forbidden(c, "only owner can delete conversation")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	_, err = tx.Exec("DELETE FROM conversation_members WHERE conversation_id = ?", convID)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete members")
		return
	}

	_, err = tx.Exec("DELETE FROM bot_conversations WHERE conversation_id = ?", convID)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete bots")
		return
	}

	_, err = tx.Exec("DELETE FROM conversations WHERE id = ?", convID)
	if err != nil {
		tx.Rollback()
		utils.InternalError(c, "failed to delete conversation")
		return
	}

	if err := tx.Commit(); err != nil {
		utils.InternalError(c, "failed to commit transaction")
		return
	}

	utils.Success(c, nil)
}

func AddMembers(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	role := getConversationRole(convID, userID)
	if role != "owner" && role != "admin" {
		utils.Forbidden(c, "only owner or admin can add members")
		return
	}

	var req AddMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	now := time.Now()
	for _, uid := range req.UserIDs {
		if isConversationMember(convID, uid) {
			continue
		}
		memberID := utils.GenerateUUID()
		database.DB.Exec(
			"INSERT INTO conversation_members (id, conversation_id, user_id, role, created_at, updated_at) VALUES (?, ?, ?, 'member', ?, ?)",
			memberID, convID, uid, now, now,
		)
	}

	database.DB.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", now, convID)

	utils.Success(c, gin.H{"message": "members added"})
}

func RemoveMember(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")
	targetUserID := c.Param("user_id")

	role := getConversationRole(convID, userID)
	targetRole := getConversationRole(convID, targetUserID)

	if targetUserID == userID {
		_, err := database.DB.Exec(
			"DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?",
			convID, userID,
		)
		if err != nil {
			utils.InternalError(c, "failed to leave conversation")
			return
		}
		utils.Success(c, nil)
		return
	}

	if role != "owner" && role != "admin" {
		utils.Forbidden(c, "only owner or admin can remove members")
		return
	}

	if targetRole == "owner" {
		utils.Forbidden(c, "cannot remove owner")
		return
	}

	if role == "admin" && targetRole == "admin" {
		utils.Forbidden(c, "admin cannot remove another admin")
		return
	}

	_, err := database.DB.Exec(
		"DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?",
		convID, targetUserID,
	)
	if err != nil {
		utils.InternalError(c, "failed to remove member")
		return
	}

	utils.Success(c, nil)
}

func UpdateMember(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")
	targetUserID := c.Param("user_id")

	role := getConversationRole(convID, userID)
	if role != "owner" {
		utils.Forbidden(c, "only owner can change member roles")
		return
	}

	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	_, err := database.DB.Exec(
		"UPDATE conversation_members SET role = ?, updated_at = ? WHERE conversation_id = ? AND user_id = ?",
		req.Role, time.Now(), convID, targetUserID,
	)
	if err != nil {
		utils.InternalError(c, "failed to update member role")
		return
	}

	utils.Success(c, nil)
}

func AddBotToConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")
	botID := c.Param("bot_id")

	role := getConversationRole(convID, userID)
	if role != "owner" && role != "admin" {
		utils.Forbidden(c, "only owner or admin can add bots")
		return
	}

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM bots WHERE id = ?)", botID).Scan(&exists)
	if err != nil || !exists {
		utils.NotFound(c, "bot not found")
		return
	}

	err = database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM bot_conversations WHERE bot_id = ? AND conversation_id = ?)",
		botID, convID,
	).Scan(&exists)
	if err == nil && exists {
		utils.BadRequest(c, "bot already in conversation")
		return
	}

	id := utils.GenerateUUID()
	now := time.Now()

	_, err = database.DB.Exec(
		"INSERT INTO bot_conversations (id, bot_id, conversation_id, added_by, created_at) VALUES (?, ?, ?, ?, ?)",
		id, botID, convID, userID, now,
	)
	if err != nil {
		utils.InternalError(c, "failed to add bot")
		return
	}

	utils.Success(c, nil)
}

func RemoveBotFromConversation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")
	botID := c.Param("bot_id")

	role := getConversationRole(convID, userID)
	if role != "owner" && role != "admin" {
		utils.Forbidden(c, "only owner or admin can remove bots")
		return
	}

	_, err := database.DB.Exec(
		"DELETE FROM bot_conversations WHERE bot_id = ? AND conversation_id = ?",
		botID, convID,
	)
	if err != nil {
		utils.InternalError(c, "failed to remove bot")
		return
	}

	utils.Success(c, nil)
}

func isConversationMember(convID, userID string) bool {
	var exists bool
	database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = ? AND user_id = ?)",
		convID, userID,
	).Scan(&exists)
	return exists
}

func getConversationRole(convID, userID string) string {
	var role string
	database.DB.QueryRow(
		"SELECT role FROM conversation_members WHERE conversation_id = ? AND user_id = ?",
		convID, userID,
	).Scan(&role)
	return role
}
