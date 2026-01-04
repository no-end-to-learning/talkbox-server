package handlers

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"talkbox/database"
	"talkbox/middleware"
	"talkbox/models"
	"talkbox/utils"
)

type CreateBotRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Avatar      string `json:"avatar"`
	Description string `json:"description" binding:"max=500"`
}

type UpdateBotRequest struct {
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
}

type BotSendMessageRequest struct {
	Type    string          `json:"type" binding:"required,oneof=text image video file card"`
	Content json.RawMessage `json:"content" binding:"required"`
}

func GetMyBots(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT id, name, avatar, description, token, created_at, updated_at
		FROM bots WHERE owner_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var bots []models.BotWithToken
	for rows.Next() {
		var bot models.Bot
		if err := rows.Scan(&bot.ID, &bot.Name, &bot.Avatar, &bot.Description, &bot.Token, &bot.CreatedAt, &bot.UpdatedAt); err != nil {
			continue
		}
		bots = append(bots, models.BotWithToken{Bot: bot, Token: bot.Token})
	}

	if bots == nil {
		bots = []models.BotWithToken{}
	}

	utils.Success(c, bots)
}

func CreateBot(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	id := utils.GenerateUUID()
	token := utils.GenerateBotToken()
	now := time.Now()

	_, err := database.DB.Exec(`
		INSERT INTO bots (id, name, avatar, description, token, owner_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.Name, req.Avatar, req.Description, token, userID, now, now)

	if err != nil {
		utils.InternalError(c, "failed to create bot")
		return
	}

	utils.Success(c, models.BotWithToken{
		Bot: models.Bot{
			ID:          id,
			Name:        req.Name,
			Avatar:      req.Avatar,
			Description: req.Description,
			OwnerID:     userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Token: token,
	})
}

func GetBot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	botID := c.Param("id")

	var bot models.Bot
	err := database.DB.QueryRow(`
		SELECT id, name, avatar, description, token, owner_id, created_at, updated_at
		FROM bots WHERE id = ? AND owner_id = ?
	`, botID, userID).Scan(&bot.ID, &bot.Name, &bot.Avatar, &bot.Description, &bot.Token, &bot.OwnerID, &bot.CreatedAt, &bot.UpdatedAt)

	if err == sql.ErrNoRows {
		utils.NotFound(c, "bot not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	utils.Success(c, models.BotWithToken{Bot: bot, Token: bot.Token})
}

func UpdateBot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	botID := c.Param("id")

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM bots WHERE id = ? AND owner_id = ?)", botID, userID).Scan(&exists)
	if err != nil || !exists {
		utils.NotFound(c, "bot not found")
		return
	}

	var req UpdateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	_, err = database.DB.Exec(`
		UPDATE bots SET
			name = COALESCE(NULLIF(?, ''), name),
			avatar = COALESCE(NULLIF(?, ''), avatar),
			description = COALESCE(NULLIF(?, ''), description),
			updated_at = ?
		WHERE id = ?
	`, req.Name, req.Avatar, req.Description, time.Now(), botID)

	if err != nil {
		utils.InternalError(c, "failed to update bot")
		return
	}

	GetBot(c)
}

func DeleteBot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	botID := c.Param("id")

	result, err := database.DB.Exec("DELETE FROM bots WHERE id = ? AND owner_id = ?", botID, userID)
	if err != nil {
		utils.InternalError(c, "failed to delete bot")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		utils.NotFound(c, "bot not found")
		return
	}

	database.DB.Exec("DELETE FROM bot_conversations WHERE bot_id = ?", botID)

	utils.Success(c, nil)
}

func RegenerateBotToken(c *gin.Context) {
	userID := middleware.GetUserID(c)
	botID := c.Param("id")

	newToken := utils.GenerateBotToken()

	result, err := database.DB.Exec(
		"UPDATE bots SET token = ?, updated_at = ? WHERE id = ? AND owner_id = ?",
		newToken, time.Now(), botID, userID,
	)
	if err != nil {
		utils.InternalError(c, "failed to regenerate token")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		utils.NotFound(c, "bot not found")
		return
	}

	utils.Success(c, gin.H{"token": newToken})
}

func GetBotConversations(c *gin.Context) {
	userID := middleware.GetUserID(c)
	botID := c.Param("id")

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM bots WHERE id = ? AND owner_id = ?)", botID, userID).Scan(&exists)
	if err != nil || !exists {
		utils.NotFound(c, "bot not found")
		return
	}

	rows, err := database.DB.Query(`
		SELECT c.id, c.type, c.name, c.avatar, c.owner_id, c.created_at
		FROM conversations c
		JOIN bot_conversations bc ON c.id = bc.conversation_id
		WHERE bc.bot_id = ?
	`, botID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var conversations []models.ConversationResponse
	for rows.Next() {
		var conv models.Conversation
		if err := rows.Scan(&conv.ID, &conv.Type, &conv.Name, &conv.Avatar, &conv.OwnerID, &conv.CreatedAt); err != nil {
			continue
		}
		conversations = append(conversations, *conv.ToResponse())
	}

	if conversations == nil {
		conversations = []models.ConversationResponse{}
	}

	utils.Success(c, conversations)
}

func BotSendMessage(c *gin.Context) {
	botID := middleware.GetBotID(c)
	convID := c.Param("conversation_id")

	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM bot_conversations WHERE bot_id = ? AND conversation_id = ?)",
		botID, convID,
	).Scan(&exists)
	if err != nil || !exists {
		utils.Forbidden(c, "bot is not a member of this conversation")
		return
	}

	var req BotSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	msgID := utils.GenerateUUID()
	now := time.Now()

	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, sender_type, type, content, created_at, updated_at)
		VALUES (?, ?, ?, 'bot', ?, ?, ?, ?)
	`, msgID, convID, botID, req.Type, string(req.Content), now, now)

	if err != nil {
		utils.InternalError(c, "failed to send message")
		return
	}

	database.DB.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", now, convID)

	utils.Success(c, gin.H{"message_id": msgID})
}
