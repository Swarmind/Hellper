package main

import (
	"hellper/internal/ai"
	"hellper/internal/database"
	"hellper/internal/telegram"
	logwrapper "hellper/pkg/log_wrapper"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	logger := log.New(os.Stderr, "Hellper ", log.LstdFlags)

	if err := godotenv.Load(); err != nil {
		logger.Printf("failed to load .env: %v", err)
	}

	db, err := database.NewHandler(loadEnv("DB_CONNECTION"))
	if err != nil {
		log.Fatalf("failed to create database service: %v", err)
	}
	ai, err := ai.NewAIService(db)
	if err != nil {
		log.Fatalf("failed to create AI service: %v", err)
	}
	tgBot, err := telegram.NewService(
		loadEnv("BOT_TOKEN"),
		db, ai,
		// We are using logwrapper, since we want to have a funcName/line data in the tg messages
		// Normally, the logger flags can provide such functionality
		&logwrapper.Service{
			Log: logger,
		})
	if err != nil {
		log.Fatalf("failed to create telegram service: %v", err)
	}

	// Can be called as non-blocking goroutine if needed
	tgBot.Start()
}

func loadEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s environment variable can't be null\n", key)
	}
	return value
}
