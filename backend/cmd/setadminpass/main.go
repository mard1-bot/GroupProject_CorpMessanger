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

	dsn := "postgres://postgres:postgres@localhost:5432/corp_messenger?sslmode=disable"
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

	_, err = db.Exec("UPDATE user_credentials SET password_hash = $1 WHERE user_id = '22222222-2222-2222-2222-222222222222'", string(hash))
	if err != nil {
		fmt.Println("update error:", err)
		os.Exit(1)
	}
	fmt.Println("Admin password updated successfully")
}
