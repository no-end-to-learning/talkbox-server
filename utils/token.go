package utils

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.New().String()
}

func GenerateBotToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}
