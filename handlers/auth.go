package handlers

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"talkbox/database"
	"talkbox/models"
	"talkbox/utils"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	Nickname string `json:"nickname"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string              `json:"token"`
	User  models.UserResponse `json:"user"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", req.Username).Scan(&exists)
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}
	if exists {
		utils.BadRequest(c, "username already exists")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalError(c, "failed to hash password")
		return
	}

	id := utils.GenerateUUID()
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}
	now := time.Now()

	_, err = database.DB.Exec(
		"INSERT INTO users (id, username, nickname, password, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, req.Username, nickname, string(hashedPassword), now, now,
	)
	if err != nil {
		utils.InternalError(c, "failed to create user")
		return
	}

	token, err := utils.GenerateToken(id)
	if err != nil {
		utils.InternalError(c, "failed to generate token")
		return
	}

	utils.Success(c, AuthResponse{
		Token: token,
		User: models.UserResponse{
			ID:       id,
			Username: req.Username,
			Nickname: nickname,
		},
	})
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var user models.User
	err := database.DB.QueryRow(
		"SELECT id, username, nickname, avatar, password FROM users WHERE username = ?",
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Nickname, &user.Avatar, &user.Password)

	if err == sql.ErrNoRows {
		utils.Unauthorized(c, "invalid username or password")
		return
	}
	if err != nil {
		utils.InternalError(c, "database error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		utils.Unauthorized(c, "invalid username or password")
		return
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		utils.InternalError(c, "failed to generate token")
		return
	}

	utils.Success(c, AuthResponse{
		Token: token,
		User:  *user.ToResponse(),
	})
}

func Logout(c *gin.Context) {
	utils.Success(c, nil)
}

func RefreshToken(c *gin.Context) {
	userID := c.GetString("user_id")

	token, err := utils.GenerateToken(userID)
	if err != nil {
		utils.InternalError(c, "failed to generate token")
		return
	}

	utils.Success(c, gin.H{"token": token})
}
