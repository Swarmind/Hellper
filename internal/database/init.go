package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type Handler struct {
	DB *sql.DB
}

func NewHandler(connectionString string) (*Handler, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_methods (
			id SERIAL PRIMARY KEY
		)`)
	if err != nil {
		log.Fatalf("failed to create default auth_methods table: %v\n", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS endpoints (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			auth_method INT REFERENCES auth_methods(id)
		)`)
	if err != nil {
		log.Fatalf("failed to create default endpoints table: %v\n", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth (
			id SERIAL PRIMARY KEY,
			tg_user_id INT NOT NULL,
			auth_method INT REFERENCES auth_methods(id),
			token TEXT NOT NULL
		)`)
	if err != nil {
		log.Fatalf("failed to create default auth table: %v\n", err)
	}

	service := Handler{
		DB: db,
	}
	return &service, nil
}
