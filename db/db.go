package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

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
		CREATE TABLE IF NOT EXISTS challenges (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			title               TEXT NOT NULL,
			description         TEXT NOT NULL,
			difficulty          TEXT NOT NULL,
			instructions        TEXT NOT NULL,
			function_name       TEXT NOT NULL,
			parameters_json     TEXT NOT NULL,
			return_type         TEXT NOT NULL,
			test_cases_json     TEXT NOT NULL,
			topic               TEXT NOT NULL,
			daily_date          TEXT NOT NULL UNIQUE,
			supported_languages TEXT NOT NULL,
			starting_code_json  TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS challenge_completions (
			user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
			completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, challenge_id)
		);
	`)
	if err != nil {
		return err
	}
	return seedDailyChallenge()
}

var ErrChallengeNotFound = errors.New("challenge not found")

func GetDailyChallenges(date string, userID int64) ([]models.Challenge, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.title, c.description, c.difficulty, c.instructions,
		       c.function_name, c.parameters_json, c.return_type, c.test_cases_json,
		       c.topic, c.daily_date, c.supported_languages, c.starting_code_json,
		       EXISTS (SELECT 1 FROM challenge_completions cc
		               WHERE cc.challenge_id = c.id AND cc.user_id = ?)
		FROM challenges c WHERE c.daily_date = ? ORDER BY c.id`, userID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Challenge
	for rows.Next() {
		var c models.Challenge
		var parameters, testCases, languages, startingCode string
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Instructions,
			&c.FunctionName, &parameters, &c.ReturnType, &testCases, &c.Topic, &c.DailyDate,
			&languages, &startingCode, &c.Solved); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(parameters), &c.Parameters); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(testCases), &c.TestCases); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(languages), &c.SupportedLanguages); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(startingCode), &c.StartingCode); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func CompleteChallenge(challengeID, userID int64) error {
	var exists bool
	if err := DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = ?)`, challengeID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrChallengeNotFound
	}
	_, err := DB.Exec(`INSERT OR IGNORE INTO challenge_completions (user_id, challenge_id) VALUES (?, ?)`, userID, challengeID)
	return err
}

func seedDailyChallenge() error {
	date := time.Now().UTC().Format("2006-01-02")

	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM challenges WHERE daily_date = ?`, date).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	parameters, _ := json.Marshal([]models.ChallengeParameter{
		{Name: "n", Type: "int"},
	})

	testCases, _ := json.Marshal([]models.ChallengeTestCase{
		{Input: json.RawMessage(`[1]`), Output: json.RawMessage(`"1"`)},
		{Input: json.RawMessage(`[3]`), Output: json.RawMessage(`"Fizz"`)},
		{Input: json.RawMessage(`[5]`), Output: json.RawMessage(`"Buzz"`)},
		{Input: json.RawMessage(`[15]`), Output: json.RawMessage(`"FizzBuzz"`)},
		{Input: json.RawMessage(`[7]`), Output: json.RawMessage(`"7"`)},
		{Input: json.RawMessage(`[30]`), Output: json.RawMessage(`"FizzBuzz"`)},
	})

	languages, _ := json.Marshal([]string{
		"java",
		"python",
		"javascript",
		"typescript",
	})

	startingCode, _ := json.Marshal(map[string]string{
		"java": `public String solve(int n) {
    // your code here
}`,
		"python": `def solve(n: int) -> str:
    pass`,
		"javascript": `function solve(n) {

}`,
		"typescript": `function solve(n: number): string {

}`,
	})

	_, err := DB.Exec(`
		INSERT INTO challenges
		(title, description, difficulty, instructions, function_name, parameters_json,
		 return_type, test_cases_json, topic, daily_date, supported_languages, starting_code_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"FizzBuzz",
		"Implement the classic FizzBuzz problem.",
		"easy",
		"Write a function `solve` that takes an integer n. Return \"FizzBuzz\" if n is divisible by both 3 and 5, \"Fizz\" if it is divisible by 3, \"Buzz\" if it is divisible by 5, and the number itself as a string otherwise.",
		"solve",
		parameters,
		"string",
		testCases,
		"fizzbuzz",
		date,
		languages,
		startingCode,
	)

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

func CreateChallenge(ch *models.Challenge) (*models.Challenge, error) {
	paramsJSON, err := json.Marshal(ch.Parameters)
	if err != nil {
		return nil, fmt.Errorf("marshal parameters: %w", err)
	}
	testCasesJSON, err := json.Marshal(ch.TestCases)
	if err != nil {
		return nil, fmt.Errorf("marshal test cases: %w", err)
	}
	langsJSON, err := json.Marshal(ch.SupportedLanguages)
	if err != nil {
		return nil, fmt.Errorf("marshal supported languages: %w", err)
	}
	startJSON, err := json.Marshal(ch.StartingCode)
	if err != nil {
		return nil, fmt.Errorf("marshal starting code: %w", err)
	}

	res, err := DB.Exec(`
		INSERT INTO challenges
		(title, description, difficulty, instructions, function_name, parameters_json,
		 return_type, test_cases_json, topic, daily_date, supported_languages, starting_code_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ch.Title, ch.Description, ch.Difficulty, ch.Instructions, ch.FunctionName,
		paramsJSON, ch.ReturnType, testCasesJSON, ch.Topic, ch.DailyDate,
		langsJSON, startJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert challenge: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	ch.ID = id
	return ch, nil
}
