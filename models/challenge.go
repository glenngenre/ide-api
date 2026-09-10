package models

import "encoding/json"

type ChallengeParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ChallengeTestCase struct {
	Input  json.RawMessage `json:"input" swaggerignore:"true"`
	Output json.RawMessage `json:"output" swaggerignore:"true"`
	Hidden bool            `json:"hidden,omitempty"`
}

type CreateChallengeRequest struct {
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

// Submission types for the /v1/challenges/{id}/submit endpoint

type TestCaseOverride struct {
	Input []any `json:"input" swaggertype:"array,object"`
}

type SubmitRequest struct {
	Language   string              `json:"language"`
	SourceCode string              `json:"source_code"`
	TestCases  []TestCaseOverride `json:"test_cases,omitempty"`
}

type CaseResult struct {
	Index    int         `json:"index"`
	Hidden   bool        `json:"hidden"`
	Passed   *bool       `json:"passed"`
	Input    []any `json:"input,omitempty" swaggertype:"array,object"`
	Expected any `json:"expected,omitempty" swaggertype:"object"`
	Actual   any `json:"actual,omitempty" swaggertype:"object"`
	Stdout   string      `json:"stdout,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type SubmitResponse struct {
	ChallengeID   int64      `json:"challenge_id"`
	Language      string     `json:"language"`
	LanguageID    int        `json:"language_id"`
	Passed        bool       `json:"passed"`
	Solved        bool       `json:"solved"`
	Status        CodeStatus `json:"status"`
	CompileOutput *string    `json:"compile_output"`
	Stderr        *string    `json:"stderr"`
	Time          string     `json:"time"`
	Memory        int        `json:"memory"`
	Total         int        `json:"total"`
	PassedCount   int        `json:"passed_count"`
	Results       []CaseResult `json:"results"`
}
