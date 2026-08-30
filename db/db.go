package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"

	"skwtr-ide-backend/models"
)

var DB *sql.DB

func Init() {
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "./data/ide.db"
	}

	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("db: failed to create data dir: %v", err)
	}

	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("db: failed to open: %v", err)
	}

	DB.SetMaxOpenConns(1) // SQLite: single writer

	if err := migrate(); err != nil {
		log.Fatalf("db: migration failed: %v", err)
	}

	log.Printf("db: ready at %s", path)
}

func migrate() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL,
			role          TEXT    NOT NULL DEFAULT 'user',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func CreateUser(username, passwordHash, role string) (*models.User, error) {
	res, err := DB.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		username, passwordHash, role,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, _ := res.LastInsertId()
	return GetUserByID(id)
}

func GetUserByUsername(username string) (*models.User, error) {
	row := DB.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`,
		username,
	)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func GetUserByID(id int64) (*models.User, error) {
	row := DB.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?`,
		id,
	)
	u := &models.User{}
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func ListUsers() ([]models.User, error) {
	rows, err := DB.Query(
		`SELECT id, username, role, created_at FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		u := models.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func DeleteUser(id int64) error {
	_, err := DB.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}
