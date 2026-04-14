package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	connStr := "postgres://postgres@localhost:5432/corp_messenger?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal("Ping failed:", err)
	}

	password := "password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Bcrypt failed:", err)
	}

	fmt.Println("Generated hash:", string(hash))

	users := []struct {
		email     string
		firstName string
		lastName  string
		role      string
	}{
		{"demo@demo.com", "Демо", "Пользователь", "user"},
		{"admin@admin.com", "Админ", "Админов", "admin"},
		{"test1@example.com", "Иван", "Тестовый", "user"},
		{"test2@example.com", "Мария", "Тестовая", "user"},
		{"test3@example.com", "Петр", "Тестовый", "user"},
		{"vladimir@example.com", "Владимир", "Пододо", "user"},
		{"arsendos@arsendo.sru", "Арсений", "Видюлин", "user"},
	}

	for _, u := range users {
		// Check if user exists
		var userID string
		err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", u.email).Scan(&userID)
		if err == sql.ErrNoRows {
			// Create user
			err = db.QueryRowContext(ctx,
				"INSERT INTO users (email, first_name, last_name, status, role, password_hash) VALUES ($1, $2, $3, 'active', $4, $5) RETURNING id",
				u.email, u.firstName, u.lastName, u.role, string(hash),
			).Scan(&userID)
			if err != nil {
				log.Printf("Failed to create user %s: %v", u.email, err)
				continue
			}
			fmt.Printf("Created user: %s (id: %s)\n", u.email, userID)
		} else if err != nil {
			log.Printf("Failed to check user %s: %v", u.email, err)
			continue
		} else {
			fmt.Printf("User exists: %s (id: %s)\n", u.email, userID)
		}

		// Upsert credentials
		_, err = db.ExecContext(ctx,
			`INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)
			 ON CONFLICT (user_id) DO UPDATE SET password_hash = $2`,
			userID, string(hash))
		if err != nil {
			log.Printf("Failed to set credentials for %s: %v", u.email, err)
			continue
		}
		fmt.Printf("Credentials set for: %s (password: %s)\n", u.email, password)
	}

	fmt.Println("\nDone! Test accounts:")
	fmt.Println("  demo@demo.com / password")
	fmt.Println("  admin@admin.com / password")
}
