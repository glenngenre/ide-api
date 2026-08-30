package models

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // "admin" | "user"
	CreatedAt    string `json:"created_at"`
}
