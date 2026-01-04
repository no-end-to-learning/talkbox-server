package handlers

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"talkbox/database"
	"talkbox/middleware"
	"talkbox/models"
	"talkbox/utils"
)

type SendMessageRequest struct {
	Type      string          `json:"type" binding:"required,oneof=text image video file card"`
	Content   json.RawMessage `json:"content" binding:"required"`
	ReplyToID string          `json:"reply_to_id"`
}

func GetMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	if !isConversationMember(convID, userID) {
		utils.Forbidden(c, "not a member of this conversation")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 100 {
		limit = 100
	}

	before := c.Query("before")

	var rows *sql.Rows
	var err error

	if before != "" {
		rows, err = database.DB.Query(`
			SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at
			FROM messages m
			WHERE m.conversation_id = ? AND m.created_at < ?
			ORDER BY m.created_at DESC
			LIMIT ?
		`, convID, before, limit)
	} else {
		rows, err = database.DB.Query(`
			SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at
			FROM messages m
			WHERE m.conversation_id = ?
			ORDER BY m.created_at DESC
			LIMIT ?
		`, convID, limit)
	}

	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var messages []models.MessageResponse
	for rows.Next() {
		var msgID, convID, senderID, senderType, msgType string
		var contentJSON []byte
		var replyToID sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&msgID, &convID, &senderID, &senderType, &msgType, &contentJSON, &replyToID, &createdAt); err != nil {
			continue
		}

		resp := models.MessageResponse{
			ID:             msgID,
			ConversationID: convID,
			Type:           msgType,
			Content:        json.RawMessage(contentJSON),
			CreatedAt:      createdAt,
		}

		if replyToID.Valid {
			resp.ReplyToID = replyToID.String
		}

		if senderType == "user" {
			var user models.User
			database.DB.QueryRow(
				"SELECT id, username, nickname, avatar FROM users WHERE id = ?",
				senderID,
			).Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar)
			resp.Sender = models.SenderInfo{
				ID:       user.ID,
				Type:     "user",
				Nickname: user.Nickname,
				Avatar:   user.Avatar,
			}
		} else {
			var bot models.Bot
			database.DB.QueryRow(
				"SELECT id, name, avatar FROM bots WHERE id = ?",
				senderID,
			).Scan(&bot.ID, &bot.Name, &bot.Avatar)
			resp.Sender = models.SenderInfo{
				ID:       bot.ID,
				Type:     "bot",
				Nickname: bot.Name,
				Avatar:   bot.Avatar,
			}
		}

		if replyToID.Valid {
			var replyMsgID, replyType string
			var replyContent []byte
			var replySenderID, replySenderType string
			err := database.DB.QueryRow(
				"SELECT id, type, content, sender_id, sender_type FROM messages WHERE id = ?",
				replyToID.String,
			).Scan(&replyMsgID, &replyType, &replyContent, &replySenderID, &replySenderType)
			if err == nil {
				reply := models.ReplyInfo{
					ID:      replyMsgID,
					Type:    replyType,
					Content: json.RawMessage(replyContent),
				}
				if replySenderType == "user" {
					var nickname string
					database.DB.QueryRow("SELECT nickname FROM users WHERE id = ?", replySenderID).Scan(&nickname)
					reply.SenderName = nickname
				} else {
					var name string
					database.DB.QueryRow("SELECT name FROM bots WHERE id = ?", replySenderID).Scan(&name)
					reply.SenderName = name
				}
				resp.ReplyTo = &reply
			}
		}

		messages = append(messages, resp)
	}

	if messages == nil {
		messages = []models.MessageResponse{}
	}

	utils.Success(c, messages)
}

func SendMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	if !isConversationMember(convID, userID) {
		utils.Forbidden(c, "not a member of this conversation")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	msgID := utils.GenerateUUID()
	now := time.Now()

	_, err := database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, sender_type, type, content, reply_to_id, created_at, updated_at)
		VALUES (?, ?, ?, 'user', ?, ?, ?, ?, ?)
	`, msgID, convID, userID, req.Type, string(req.Content), sql.NullString{String: req.ReplyToID, Valid: req.ReplyToID != ""}, now, now)

	if err != nil {
		utils.InternalError(c, "failed to send message")
		return
	}

	if req.Type == "text" {
		var textContent models.TextContent
		if err := json.Unmarshal(req.Content, &textContent); err == nil && len(textContent.Mentions) > 0 {
			for _, mentionedUserID := range textContent.Mentions {
				mentionID := utils.GenerateUUID()
				database.DB.Exec(
					"INSERT INTO mentions (id, message_id, user_id, created_at) VALUES (?, ?, ?, ?)",
					mentionID, msgID, mentionedUserID, now,
				)
			}
		}
	}

	database.DB.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", now, convID)

	utils.Success(c, gin.H{"message_id": msgID})
}

func SearchMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	convID := c.Param("id")

	if !isConversationMember(convID, userID) {
		utils.Forbidden(c, "not a member of this conversation")
		return
	}

	query := c.Query("q")
	if query == "" {
		utils.BadRequest(c, "search query is required")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 50 {
		limit = 50
	}

	rows, err := database.DB.Query(`
		SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at
		FROM messages m
		WHERE m.conversation_id = ? AND m.type = 'text' AND MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)
		ORDER BY m.created_at DESC
		LIMIT ?
	`, convID, query, limit)

	if err != nil {
		rows, err = database.DB.Query(`
			SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at
			FROM messages m
			WHERE m.conversation_id = ? AND m.type = 'text' AND m.content LIKE ?
			ORDER BY m.created_at DESC
			LIMIT ?
		`, convID, "%"+query+"%", limit)
		if err != nil {
			utils.InternalError(c, "database error")
			return
		}
	}
	defer rows.Close()

	var messages []models.MessageResponse
	for rows.Next() {
		var msgID, cID, senderID, senderType, msgType string
		var contentJSON []byte
		var replyToID sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&msgID, &cID, &senderID, &senderType, &msgType, &contentJSON, &replyToID, &createdAt); err != nil {
			continue
		}

		resp := models.MessageResponse{
			ID:             msgID,
			ConversationID: cID,
			Type:           msgType,
			Content:        json.RawMessage(contentJSON),
			CreatedAt:      createdAt,
		}

		if replyToID.Valid {
			resp.ReplyToID = replyToID.String
		}

		if senderType == "user" {
			var user models.User
			database.DB.QueryRow(
				"SELECT id, username, nickname, avatar FROM users WHERE id = ?",
				senderID,
			).Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar)
			resp.Sender = models.SenderInfo{
				ID:       user.ID,
				Type:     "user",
				Nickname: user.Nickname,
				Avatar:   user.Avatar,
			}
		} else {
			var bot models.Bot
			database.DB.QueryRow(
				"SELECT id, name, avatar FROM bots WHERE id = ?",
				senderID,
			).Scan(&bot.ID, &bot.Name, &bot.Avatar)
			resp.Sender = models.SenderInfo{
				ID:       bot.ID,
				Type:     "bot",
				Nickname: bot.Name,
				Avatar:   bot.Avatar,
			}
		}

		messages = append(messages, resp)
	}

	if messages == nil {
		messages = []models.MessageResponse{}
	}

	utils.Success(c, messages)
}

func StartPrivateChat(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var isFriend bool
	database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM friendships WHERE user_id = ? AND friend_id = ? AND status = 'accepted')",
		userID, req.UserID,
	).Scan(&isFriend)

	if !isFriend {
		utils.Forbidden(c, "can only chat with friends")
		return
	}

	convID, err := FindOrCreatePrivateConversation(userID, req.UserID)
	if err != nil {
		utils.InternalError(c, "failed to create conversation")
		return
	}

	utils.Success(c, gin.H{"conversation_id": convID})
}
