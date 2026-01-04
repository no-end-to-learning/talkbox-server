package database

import (
	"database/sql"
	"log"
	"talkbox/config"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func Connect() error {
	var err error
	DB, err = sql.Open("mysql", config.Cfg.MysqlDSN)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	log.Println("Database connected successfully")
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}

func CreateTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id          VARCHAR(36) PRIMARY KEY,
			username    VARCHAR(50) NOT NULL,
			nickname    VARCHAR(100),
			avatar      VARCHAR(255),
			password    VARCHAR(255) NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_username (username)
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id          VARCHAR(36) PRIMARY KEY,
			type        ENUM('private', 'group') NOT NULL,
			name        VARCHAR(100),
			avatar      VARCHAR(255),
			owner_id    VARCHAR(36),
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_owner (owner_id)
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_members (
			id              VARCHAR(36) PRIMARY KEY,
			conversation_id VARCHAR(36) NOT NULL,
			user_id         VARCHAR(36) NOT NULL,
			role            ENUM('owner', 'admin', 'member') DEFAULT 'member',
			nickname        VARCHAR(100),
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_conv_user (conversation_id, user_id),
			INDEX idx_user (user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id              VARCHAR(36) PRIMARY KEY,
			conversation_id VARCHAR(36) NOT NULL,
			sender_id       VARCHAR(36) NOT NULL,
			sender_type     ENUM('user', 'bot') DEFAULT 'user',
			type            ENUM('text', 'image', 'video', 'file', 'card') NOT NULL,
			content         JSON NOT NULL,
			reply_to_id     VARCHAR(36),
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_conv_time (conversation_id, created_at),
			INDEX idx_reply (reply_to_id)
		)`,
		`CREATE TABLE IF NOT EXISTS mentions (
			id          VARCHAR(36) PRIMARY KEY,
			message_id  VARCHAR(36) NOT NULL,
			user_id     VARCHAR(36) NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_message (message_id),
			INDEX idx_user (user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS bots (
			id          VARCHAR(36) PRIMARY KEY,
			name        VARCHAR(100) NOT NULL,
			avatar      VARCHAR(255),
			description VARCHAR(500),
			token       VARCHAR(64) NOT NULL,
			owner_id    VARCHAR(36) NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_token (token),
			INDEX idx_owner (owner_id)
		)`,
		`CREATE TABLE IF NOT EXISTS bot_conversations (
			id              VARCHAR(36) PRIMARY KEY,
			bot_id          VARCHAR(36) NOT NULL,
			conversation_id VARCHAR(36) NOT NULL,
			added_by        VARCHAR(36) NOT NULL,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_bot_conv (bot_id, conversation_id),
			INDEX idx_conv (conversation_id)
		)`,
		`CREATE TABLE IF NOT EXISTS device_tokens (
			id          VARCHAR(36) PRIMARY KEY,
			user_id     VARCHAR(36) NOT NULL,
			platform    ENUM('ios', 'android') NOT NULL,
			token       VARCHAR(255) NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_user_platform (user_id, platform)
		)`,
		`CREATE TABLE IF NOT EXISTS friendships (
			id          VARCHAR(36) PRIMARY KEY,
			user_id     VARCHAR(36) NOT NULL,
			friend_id   VARCHAR(36) NOT NULL,
			status      ENUM('pending', 'accepted', 'blocked') DEFAULT 'pending',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_friendship (user_id, friend_id),
			INDEX idx_friend (friend_id)
		)`,
	}

	for _, table := range tables {
		if _, err := DB.Exec(table); err != nil {
			return err
		}
	}

	log.Println("Database tables created successfully")
	return nil
}
