package challenges

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var judge0Base string

func init() {
	judge0Base = strings.TrimRight(os.Getenv("JUDGE0_BASE_URL"), "/")
	if judge0Base == "" {
		judge0Base = "https://judge0.apps.skwtr.com"
	}
}

func authHeader() string {
	return strings.TrimSpace(os.Getenv("JUDGE0_AUTHN_TOKEN"))
}

type batchSubmission struct {
	SourceCode             string `json:"source_code"`
	LanguageID             int    `json:"language_id"`
	Stdin                  string `json:"stdin"`
	CPUTimeLimit           int    `json:"cpu_time_limit"`
	RedirectStderrToStdout bool   `json:"redirect_stderr_to_stdout"`
}

type batchToken struct {
	Token string `json:"token"`
}

type Judge0Result struct {
	Token  string `json:"token"`
	Status struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	CompileOutput string `json:"compile_output"`
	Time          string `json:"time"`
	Memory        int    `json:"memory"`
}

func SubmitBatch(languageID int, sourceCode string, stdinCases []string) ([]Judge0Result, error) {
	if len(stdinCases) == 0 {
		return []Judge0Result{}, nil
	}

	submissions := make([]batchSubmission, len(stdinCases))
	for i, stdin := range stdinCases {
		submissions[i] = batchSubmission{
			SourceCode:             sourceCode,
			LanguageID:             languageID,
			Stdin:                  stdin,
			CPUTimeLimit:           5,
			RedirectStderrToStdout: false,
		}
	}

	body, err := json.Marshal(map[string]any{"submissions": submissions})
	if err != nil {
		return nil, fmt.Errorf("marshal batch: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, judge0Base+"/submissions/batch?base64_encoded=false", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if auth := authHeader(); auth != "" {
		req.Header.Set("X-Auth-Token", auth)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit batch: %w", err)
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("judge0 batch error %d: %s", resp.StatusCode, responseBody)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read batch response: %w", readErr)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(responseBody, &items); err != nil {
		return nil, fmt.Errorf("decode batch tokens: %w; response: %s", err, responseBody)
	}
	if len(items) != len(submissions) {
		return nil, fmt.Errorf("judge0 batch returned %d items, want %d tokens; response: %s", len(items), len(submissions), responseBody)
	}

	tokenStrs := make([]string, len(items))
	seen := make(map[string]int, len(items))
	for i, item := range items {
		var t batchToken
		if err := json.Unmarshal(item, &t); err != nil {
			return nil, fmt.Errorf("decode batch token for submission %d: %w; response: %s", i+1, err, responseBody)
		}
		if strings.TrimSpace(t.Token) == "" {
			return nil, fmt.Errorf("judge0 batch submission %d returned no token (possible validation error); response: %s", i+1, responseBody)
		}
		if previous, ok := seen[t.Token]; ok {
			return nil, fmt.Errorf("judge0 batch returned duplicate token %q for submissions %d and %d; response: %s", t.Token, previous+1, i+1, responseBody)
		}
		seen[t.Token] = i
		tokenStrs[i] = t.Token
	}

	return pollBatch(tokenStrs)
}

func pollBatch(tokens []string) ([]Judge0Result, error) {
	if len(tokens) == 0 {
		return []Judge0Result{}, nil
	}

	indices := make(map[string]int, len(tokens))
	for i, token := range tokens {
		if strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("poll batch: missing token for submission %d", i+1)
		}
		if previous, ok := indices[token]; ok {
			return nil, fmt.Errorf("poll batch: duplicate token %q for submissions %d and %d", token, previous+1, i+1)
		}
		indices[token] = i
	}

	joined := url.QueryEscape(strings.Join(tokens, ","))
	pollURL := judge0Base + "/submissions/batch?tokens=" + joined + "&base64_encoded=true"

	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)

		req, err := http.NewRequest(http.MethodGet, pollURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if auth := authHeader(); auth != "" {
			req.Header.Set("X-Auth-Token", auth)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		responseBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("judge0 batch poll error %d: %s", resp.StatusCode, responseBody)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read batch poll response: %w", readErr)
		}

		var wrapper struct {
			Submissions []Judge0Result `json:"submissions"`
		}
		if err := json.Unmarshal(responseBody, &wrapper); err != nil {
			return nil, fmt.Errorf("decode batch poll response: %w", err)
		}
		if len(wrapper.Submissions) != len(tokens) {
			return nil, fmt.Errorf("judge0 batch poll returned %d submissions, want %d", len(wrapper.Submissions), len(tokens))
		}

		results := make([]Judge0Result, len(tokens))
		seen := make([]bool, len(tokens))
		allDone := true
		for i, r := range wrapper.Submissions {
			if strings.TrimSpace(r.Token) == "" {
				return nil, fmt.Errorf("judge0 batch poll submission %d has no token", i+1)
			}
			index, ok := indices[r.Token]
			if !ok {
				return nil, fmt.Errorf("judge0 batch poll returned unexpected token %q", r.Token)
			}
			if seen[index] {
				return nil, fmt.Errorf("judge0 batch poll returned duplicate token %q", r.Token)
			}
			seen[index] = true

			// Judge0 status IDs: 1 = In Queue, 2 = Processing, 3–14 = terminal.
			if r.Status.ID < 1 || r.Status.ID > 14 {
				return nil, fmt.Errorf("judge0 batch poll token %q has invalid status %d (%q)", r.Token, r.Status.ID, r.Status.Description)
			}
			if r.Status.ID == 1 || r.Status.ID == 2 {
				allDone = false
			}
			results[index] = r
		}
		if allDone {
			for i := range results {
				r := &results[i]
				for _, field := range []struct {
					name  string
					value *string
				}{
					{"stdout", &r.Stdout},
					{"stderr", &r.Stderr},
					{"compile_output", &r.CompileOutput},
				} {
					decoded, err := decodeField(*field.value)
					if err != nil {
						return nil, fmt.Errorf("decode batch token %q %s: %w", r.Token, field.name, err)
					}
					*field.value = decoded
				}
			}
			return results, nil
		}
	}

	return nil, fmt.Errorf("polling timeout")
}

func decodeField(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
