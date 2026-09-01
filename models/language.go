package models

type Language struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Version  string `json:"version,omitempty"`
}
