package handlers

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
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

// escapeLikePattern 转义 SQL LIKE 查询中的特殊字符
func escapeLikePattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, "\\", "\\\\")
	pattern = strings.ReplaceAll(pattern, "%", "\\%")
	pattern = strings.ReplaceAll(pattern, "_", "\\_")
	return pattern
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

	// 使用 LEFT JOIN 一次性获取消息和发送者信息，解决 N+1 问题
	baseQuery := `
		SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at,
			   COALESCE(u.id, '') as user_id, COALESCE(u.username, '') as username, COALESCE(u.nickname, '') as user_nickname, COALESCE(u.avatar, '') as user_avatar,
			   COALESCE(b.id, '') as bot_id, COALESCE(b.name, '') as bot_name, COALESCE(b.avatar, '') as bot_avatar
		FROM messages m
		LEFT JOIN users u ON m.sender_type = 'user' AND m.sender_id = u.id
		LEFT JOIN bots b ON m.sender_type = 'bot' AND m.sender_id = b.id
		WHERE m.conversation_id = ?`

	if before != "" {
		rows, err = database.DB.Query(baseQuery+` AND m.created_at < ? ORDER BY m.created_at DESC LIMIT ?`, convID, before, limit)
	} else {
		rows, err = database.DB.Query(baseQuery+` ORDER BY m.created_at DESC LIMIT ?`, convID, limit)
	}

	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	// 收集所有消息和需要查询回复的消息ID
	var messages []models.MessageResponse
	var replyIDs []string
	replyIDMap := make(map[string]int) // replyID -> message index

	for rows.Next() {
		var msgID, cID, senderID, senderType, msgType string
		var contentJSON []byte
		var replyToID sql.NullString
		var createdAt time.Time
		var userID, username, userNickname, userAvatar string
		var botID, botName, botAvatar string

		if err := rows.Scan(&msgID, &cID, &senderID, &senderType, &msgType, &contentJSON, &replyToID, &createdAt,
			&userID, &username, &userNickname, &userAvatar, &botID, &botName, &botAvatar); err != nil {
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
			replyIDMap[replyToID.String] = len(messages)
			replyIDs = append(replyIDs, replyToID.String)
		}

		if senderType == "user" {
			resp.Sender = models.SenderInfo{
				ID:       userID,
				Type:     "user",
				Nickname: userNickname,
				Avatar:   userAvatar,
			}
		} else {
			resp.Sender = models.SenderInfo{
				ID:       botID,
				Type:     "bot",
				Nickname: botName,
				Avatar:   botAvatar,
			}
		}

		messages = append(messages, resp)
	}

	// 批量查询回复消息信息
	if len(replyIDs) > 0 {
		placeholders := strings.Repeat("?,", len(replyIDs)-1) + "?"
		args := make([]interface{}, len(replyIDs))
		for i, id := range replyIDs {
			args[i] = id
		}

		replyRows, err := database.DB.Query(`
			SELECT m.id, m.type, m.content, m.sender_id, m.sender_type,
				   COALESCE(u.nickname, '') as user_nickname,
				   COALESCE(b.name, '') as bot_name
			FROM messages m
			LEFT JOIN users u ON m.sender_type = 'user' AND m.sender_id = u.id
			LEFT JOIN bots b ON m.sender_type = 'bot' AND m.sender_id = b.id
			WHERE m.id IN (`+placeholders+`)
		`, args...)

		if err == nil {
			defer replyRows.Close()
			for replyRows.Next() {
				var replyMsgID, replyType, replySenderID, replySenderType string
				var replyContent []byte
				var userNickname, botName string

				if err := replyRows.Scan(&replyMsgID, &replyType, &replyContent, &replySenderID, &replySenderType, &userNickname, &botName); err != nil {
					continue
				}

				if idx, ok := replyIDMap[replyMsgID]; ok {
					reply := models.ReplyInfo{
						ID:      replyMsgID,
						Type:    replyType,
						Content: json.RawMessage(replyContent),
					}
					if replySenderType == "user" {
						reply.SenderName = userNickname
					} else {
						reply.SenderName = botName
					}
					messages[idx].ReplyTo = &reply
				}
			}
		}
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
		// 转义 LIKE 特殊字符防止意外匹配
		escapedQuery := "%" + escapeLikePattern(query) + "%"
		rows, err = database.DB.Query(`
			SELECT m.id, m.conversation_id, m.sender_id, m.sender_type, m.type, m.content, m.reply_to_id, m.created_at
			FROM messages m
			WHERE m.conversation_id = ? AND m.type = 'text' AND m.content LIKE ? ESCAPE '\\'
			ORDER BY m.created_at DESC
			LIMIT ?
		`, convID, escapedQuery, limit)
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
