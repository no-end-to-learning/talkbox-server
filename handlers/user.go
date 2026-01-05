package handlers

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"talkbox/config"
	"talkbox/database"
	"talkbox/middleware"
	"talkbox/models"
	"talkbox/utils"
)

type UpdateUserRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type DeviceTokenRequest struct {
	Platform string `json:"platform" binding:"required,oneof=ios android"`
	Token    string `json:"token" binding:"required"`
}

func GetCurrentUser(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	err := database.DB.QueryRow(
		"SELECT id, username, nickname, avatar, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar, &user.CreatedAt)

	if err == sql.ErrNoRows {
		utils.NotFound(c, "user not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	utils.Success(c, user.ToResponse())
}

func UpdateCurrentUser(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	_, err := database.DB.Exec(
		"UPDATE users SET nickname = COALESCE(NULLIF(?, ''), nickname), avatar = COALESCE(NULLIF(?, ''), avatar), updated_at = ? WHERE id = ?",
		req.Nickname, req.Avatar, time.Now(), userID,
	)
	if err != nil {
		utils.InternalError(c, "failed to update user")
		return
	}

	GetCurrentUser(c)
}

func UploadAvatar(c *gin.Context) {
	userID := middleware.GetUserID(c)

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		utils.BadRequest(c, "no file uploaded")
		return
	}
	defer file.Close()

	// 限制头像大小为 2MB
	maxSize := int64(2 * 1024 * 1024)
	if header.Size > maxSize {
		utils.BadRequest(c, "avatar too large (max 2MB)")
		return
	}

	// 验证文件类型为图片
	mimeType := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowedTypes[mimeType] {
		utils.BadRequest(c, "avatar must be an image (jpeg, png, gif, webp)")
		return
	}

	ext := filepath.Ext(header.Filename)
	filename := utils.GenerateUUID() + ext
	uploadPath := filepath.Join(config.Cfg.UploadDir, filename)

	out, err := os.Create(uploadPath)
	if err != nil {
		utils.InternalError(c, "failed to save file")
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		utils.InternalError(c, "failed to save file")
		return
	}

	avatarURL := "/files/" + filename
	_, err = database.DB.Exec(
		"UPDATE users SET avatar = ?, updated_at = ? WHERE id = ?",
		avatarURL, time.Now(), userID,
	)
	if err != nil {
		utils.InternalError(c, "failed to update avatar")
		return
	}

	utils.Success(c, gin.H{"avatar": avatarURL})
}

func RegisterDeviceToken(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req DeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	id := utils.GenerateUUID()
	now := time.Now()

	_, err := database.DB.Exec(`
		INSERT INTO device_tokens (id, user_id, platform, token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE token = ?, updated_at = ?
	`, id, userID, req.Platform, req.Token, now, now, req.Token, now)

	if err != nil {
		utils.InternalError(c, "failed to register device token")
		return
	}

	utils.Success(c, nil)
}

func UnregisterDeviceToken(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req DeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	_, err := database.DB.Exec(
		"DELETE FROM device_tokens WHERE user_id = ? AND platform = ?",
		userID, req.Platform,
	)
	if err != nil {
		utils.InternalError(c, "failed to unregister device token")
		return
	}

	utils.Success(c, nil)
}

func GetAllUsers(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT id, username, nickname, avatar FROM users
		WHERE id != ?
		ORDER BY nickname, username
	`, userID)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var users []models.UserResponse
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar); err != nil {
			continue
		}
		users = append(users, *user.ToResponse())
	}

	if users == nil {
		users = []models.UserResponse{}
	}

	utils.Success(c, users)
}

func SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		utils.BadRequest(c, "search query is required")
		return
	}

	userID := middleware.GetUserID(c)

	rows, err := database.DB.Query(`
		SELECT id, username, nickname, avatar FROM users
		WHERE id != ? AND (username LIKE ? OR nickname LIKE ?)
		LIMIT 20
	`, userID, "%"+query+"%", "%"+query+"%")
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	defer rows.Close()

	var users []models.UserResponse
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar); err != nil {
			continue
		}
		users = append(users, *user.ToResponse())
	}

	if users == nil {
		users = []models.UserResponse{}
	}

	utils.Success(c, users)
}
