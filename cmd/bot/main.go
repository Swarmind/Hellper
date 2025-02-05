package main

import (
	"hellper/internal/ai"
	"hellper/internal/database"
	"hellper/internal/telegram"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("failed to load .env: %v\n", err)
	}

	db, err := database.NewHandler(loadEnv("DB_CONNECTION"))
	if err != nil {
		log.Fatalf("failed to create database service: %v\n", err)
	}
	ai := ai.Service{
		UsersRuntimeCache: sync.Map{},
	}
	tgBot, err := telegram.NewService(loadEnv("BOT_TOKEN"), db, &ai)
	if err != nil {
		log.Fatalf("failed to create telegram service: %v\n", err)
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
