package middleware

import (
	"database/sql"
	"strings"

	"github.com/gin-gonic/gin"
	"talkbox/database"
	"talkbox/utils"
)

func BotAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.Unauthorized(c, "invalid authorization header format")
			c.Abort()
			return
		}

		token := parts[1]

		var botID string
		err := database.DB.QueryRow("SELECT id FROM bots WHERE token = ?", token).Scan(&botID)
		if err == sql.ErrNoRows {
			utils.Unauthorized(c, "invalid bot token")
			c.Abort()
			return
		}
		if err != nil {
			utils.InternalError(c, "database error")
			c.Abort()
			return
		}

		c.Set("bot_id", botID)
		c.Next()
	}
}

func GetBotID(c *gin.Context) string {
	botID, _ := c.Get("bot_id")
	return botID.(string)
}
