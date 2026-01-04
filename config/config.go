package config

import (
	"os"
)

type Config struct {
	ServerAddr string
	MysqlDSN   string
	JWTSecret  string
	UploadDir  string
}

var Cfg *Config

func Load() {
	Cfg = &Config{
		ServerAddr: ":" + getEnv("PORT", "8080"),
		MysqlDSN:   getEnv("MYSQL_DSN", "root:root@tcp(localhost:3306)/talkbox?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:  getEnv("JWT_SECRET", "talkbox-secret-key-change-in-production"),
		UploadDir:  getEnv("UPLOAD_DIR", "./uploads"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
