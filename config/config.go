package config

import (
	"log"
	"os"
)

type Config struct {
	ServerAddr     string
	MysqlDSN       string
	JWTSecret      string
	UploadDir      string
	AllowedOrigins string
}

var Cfg *Config

func Load() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		log.Fatal("MYSQL_DSN environment variable is required")
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		log.Fatal("UPLOAD_DIR environment variable is required")
	}

	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal("CORS_ALLOWED_ORIGINS environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("PORT environment variable is required")
	}

	Cfg = &Config{
		ServerAddr:     ":" + port,
		MysqlDSN:       mysqlDSN,
		JWTSecret:      jwtSecret,
		UploadDir:      uploadDir,
		AllowedOrigins: allowedOrigins,
	}
}
