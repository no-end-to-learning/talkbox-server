package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"talkbox/config"
	"talkbox/database"
	"talkbox/handlers"
	"talkbox/middleware"
	"talkbox/websocket"
)

func main() {
	config.Load()

	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.DB.Close()

	if err := database.CreateTables(); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	if err := os.MkdirAll(config.Cfg.UploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	websocket.InitHub()

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/logout", middleware.AuthMiddleware(), handlers.Logout)
		auth.POST("/refresh", middleware.AuthMiddleware(), handlers.RefreshToken)
	}

	users := r.Group("/api/users")
	users.Use(middleware.AuthMiddleware())
	{
		users.GET("", handlers.GetAllUsers)
		users.GET("/me", handlers.GetCurrentUser)
		users.PUT("/me", handlers.UpdateCurrentUser)
		users.POST("/me/avatar", handlers.UploadAvatar)
		users.POST("/me/device", handlers.RegisterDeviceToken)
		users.DELETE("/me/device", handlers.UnregisterDeviceToken)
		users.GET("/search", handlers.SearchUsers)
	}

	friends := r.Group("/api/friends")
	friends.Use(middleware.AuthMiddleware())
	{
		friends.GET("", handlers.GetFriends)
		friends.GET("/requests", handlers.GetFriendRequests)
		friends.POST("/request", handlers.SendFriendRequest)
		friends.POST("/accept/:user_id", handlers.AcceptFriendRequest)
		friends.DELETE("/:user_id", handlers.DeleteFriend)
	}

	conversations := r.Group("/api/conversations")
	conversations.Use(middleware.AuthMiddleware())
	{
		conversations.GET("", handlers.GetConversations)
		conversations.POST("", handlers.CreateConversation)
		conversations.POST("/private", handlers.StartPrivateChat)
		conversations.GET("/:id", handlers.GetConversation)
		conversations.PUT("/:id", handlers.UpdateConversation)
		conversations.DELETE("/:id", handlers.DeleteConversation)

		conversations.POST("/:id/members", handlers.AddMembers)
		conversations.DELETE("/:id/members/:user_id", handlers.RemoveMember)
		conversations.PUT("/:id/members/:user_id", handlers.UpdateMember)

		conversations.POST("/:id/bots/:bot_id", handlers.AddBotToConversation)
		conversations.DELETE("/:id/bots/:bot_id", handlers.RemoveBotFromConversation)

		conversations.GET("/:id/messages", handlers.GetMessages)
		conversations.POST("/:id/messages", handlers.SendMessage)
		conversations.GET("/:id/messages/search", handlers.SearchMessages)
	}

	files := r.Group("/api/files")
	files.Use(middleware.AuthMiddleware())
	{
		files.POST("/upload", handlers.UploadFile)
	}

	r.GET("/files/:filename", handlers.ServeFile)

	bots := r.Group("/api/bots")
	bots.Use(middleware.AuthMiddleware())
	{
		bots.GET("", handlers.GetMyBots)
		bots.POST("", handlers.CreateBot)
		bots.GET("/:id", handlers.GetBot)
		bots.PUT("/:id", handlers.UpdateBot)
		bots.DELETE("/:id", handlers.DeleteBot)
		bots.POST("/:id/token", handlers.RegenerateBotToken)
		bots.GET("/:id/conversations", handlers.GetBotConversations)
	}

	botAPI := r.Group("/api/bot")
	botAPI.Use(middleware.BotAuthMiddleware())
	{
		botAPI.POST("/conversations/:conversation_id/messages", handlers.BotSendMessage)
	}

	r.GET("/ws", websocket.HandleWebSocket)

	log.Printf("Server starting on %s", config.Cfg.ServerAddr)
	if err := r.Run(config.Cfg.ServerAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
