package models

import "encoding/json"

type ChallengeParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ChallengeTestCase struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

type Challenge struct {
	ID                 int64                `json:"id"`
	Title              string               `json:"title"`
	Description        string               `json:"description"`
	Difficulty         string               `json:"difficulty"`
	Instructions       string               `json:"instructions"`
	FunctionName       string               `json:"function_name"`
	Parameters         []ChallengeParameter `json:"parameters"`
	ReturnType         string               `json:"return_type"`
	TestCases          []ChallengeTestCase  `json:"test_cases"`
	Topic              string               `json:"topic"`
	DailyDate          string               `json:"daily_date"`
	SupportedLanguages []string             `json:"supported_languages"`
	StartingCode       map[string]string    `json:"starting_code"`
	Solved             bool                 `json:"solved"`
}
