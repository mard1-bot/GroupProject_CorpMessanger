package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: setadminpass <new-password>")
		fmt.Println("Example: setadminpass 'SecurePassword123!'")
		os.Exit(1)
	}

	password := os.Args[1]
	if len(password) < 8 {
		fmt.Println("Error: Password must be at least 8 characters")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Default local development DSN
		dsn = "postgres://postgres:postgres@localhost:5432/corp_messenger?sslmode=disable"
	}
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println("DB error:", err)
		os.Exit(1)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("bcrypt error:", err)
		os.Exit(1)
	}

	// Find the admin user ID by email
	var userID string
	err = db.QueryRow("SELECT id FROM users WHERE email = 'admin@corpmessenger.com'").Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("Error: Default admin user (admin@corpmessenger.com) not found. Has the database been initialized?")
		} else {
			fmt.Println("Error finding admin user:", err)
		}
		os.Exit(1)
	}

	// Upsert the password into user_credentials
	_, err = db.Exec(`
		INSERT INTO user_credentials (user_id, password_hash) 
		VALUES ($1, $2)
		ON CONFLICT (user_id) 
		DO UPDATE SET password_hash = EXCLUDED.password_hash
	`, userID, string(hash))
	
	if err != nil {
		fmt.Println("update error:", err)
		os.Exit(1)
	}
	fmt.Println("Admin password updated successfully for admin@corpmessenger.com")
}
