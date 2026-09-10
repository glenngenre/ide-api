package challenges

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"skwtr-ide-backend/models"
)

type judge0Submission struct {
	SourceCode             string `json:"source_code"`
	LanguageID             int    `json:"language_id"`
	RedirectStderrToStdout bool   `json:"redirect_stderr_to_stdout"`
	CPUTimeLimit           int    `json:"cpu_time_limit"`
}

type judge0Response struct {
	Token  string `json:"token"`
	Status struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
	CompileOutput string `json:"compile_output"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	Time          string `json:"time"`
	Memory        int    `json:"memory"`
}

type HarnessResult struct {
	Index  int             `json:"i"`
	Ok     bool            `json:"ok"`
	Out    json.RawMessage `json:"out"`
	Err    string          `json:"err"`
	Stdout string          `json:"stdout"`
}

var judge0Base string

func init() {
	judge0Base = strings.TrimRight(os.Getenv("JUDGE0_BASE_URL"), "/")
	if judge0Base == "" {
		judge0Base = "https://judge0.apps.skwtr.com"
	}
}

func SubmitToJudge0(languageID int, sourceCode string, caseCount int) (*judge0Response, error) {
	cpuTimeLimit := min(2 + (caseCount / 2), 15)

	submission := judge0Submission{
		SourceCode:             sourceCode,
		LanguageID:             languageID,
		RedirectStderrToStdout: false,
		CPUTimeLimit:           cpuTimeLimit,
	}

	submissionJSON, err := json.Marshal(submission)
	if err != nil {
		return nil, fmt.Errorf("marshal submission: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		judge0Base+"/submissions?base64_encoded=true&wait=false",
		bytes.NewReader(submissionJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if auth := strings.TrimSpace(os.Getenv("JUDGE0_AUTHN_TOKEN")); auth != "" {
		req.Header.Set("X-Auth-Token", auth)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit to judge0: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("judge0 error: status %d: %s", resp.StatusCode, string(body))
	}

	var submitResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		return nil, fmt.Errorf("decode submission response: %w", err)
	}

	return pollForCompletion(submitResp.Token)
}

func pollForCompletion(token string) (*judge0Response, error) {
	maxAttempts := 60
	attempt := 0

	for attempt < maxAttempts {
		time.Sleep(500 * time.Millisecond)

		req, err := http.NewRequest(
			http.MethodGet,
			judge0Base+"/submissions/"+token+"?base64_encoded=true",
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("create poll request: %w", err)
		}

		req.Header.Set("Accept", "application/json")
		if auth := strings.TrimSpace(os.Getenv("JUDGE0_AUTHN_TOKEN")); auth != "" {
			req.Header.Set("X-Auth-Token", auth)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			attempt++
			continue
		}
		defer resp.Body.Close()

		var result judge0Response
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			attempt++
			continue
		}

		if result.Status.ID == 1 || result.Status.ID == 2 {
			attempt++
			continue
		}

		if result.Stdout != "" {
			decoded, err := base64.StdEncoding.DecodeString(result.Stdout)
			if err == nil {
				result.Stdout = string(decoded)
			}
		}
		if result.Stderr != "" {
			decoded, err := base64.StdEncoding.DecodeString(result.Stderr)
			if err == nil {
				result.Stderr = string(decoded)
			}
		}
		if result.CompileOutput != "" {
			decoded, err := base64.StdEncoding.DecodeString(result.CompileOutput)
			if err == nil {
				result.CompileOutput = string(decoded)
			}
		}

		return &result, nil
	}

	return nil, fmt.Errorf("polling timeout after %d attempts", maxAttempts)
}

func ParseHarnessOutput(stdout string) ([]HarnessResult, string, error) {
	var results []HarnessResult
	var harness_errors []string

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if after, ok :=strings.CutPrefix(line, "__SKWTR__ "); ok  {
			jsonStr := after
			var result HarnessResult
			if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
				harness_errors = append(harness_errors, fmt.Sprintf("Failed to parse result: %v", err))
				continue
			}
			results = append(results, result)
		} else {
			harness_errors = append(harness_errors, line)
		}
	}

	stderrMsg := strings.Join(harness_errors, "\n")
	return results, stderrMsg, nil
}

func BuildAndTestChallenge(
	challenge *models.Challenge,
	language string,
	sourceCode string,
	testCaseOverrides []models.TestCaseOverride,
) ([]HarnessResult, *judge0Response, string, error) {
	langCfg := GetLanguageConfig(language)
	if langCfg == nil {
		return nil, nil, "", fmt.Errorf("unsupported language: %s", language)
	}

	var testCases []models.ChallengeTestCase
	if len(testCaseOverrides) > 0 {
		testCases = convertOverridesToTestCases(testCaseOverrides)
	} else {
		testCases = challenge.TestCases
	}

	var casesArray []any
	for _, tc := range testCases {
		var inputArray []any
		if err := json.Unmarshal(tc.Input, &inputArray); err != nil {
			return nil, nil, "", fmt.Errorf("parse test case input: %w", err)
		}
		casesArray = append(casesArray, inputArray)
	}

	casesJSON, err := json.Marshal(casesArray)
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal cases: %w", err)
	}

	fullSource, err := langCfg.Builder.Build(challenge.FunctionName, sourceCode, string(casesJSON))
	if err != nil {
		return nil, nil, "", fmt.Errorf("build harness: %w", err)
	}

	judge0Result, err := SubmitToJudge0(langCfg.Judge0ID, fullSource, len(testCases))
	if err != nil {
		return nil, nil, "", fmt.Errorf("submit to judge0: %w", err)
	}

	results, stderrMsg, err := ParseHarnessOutput(judge0Result.Stdout)
	if err != nil {
		return nil, judge0Result, stderrMsg, fmt.Errorf("parse harness output: %w", err)
	}

	return results, judge0Result, stderrMsg, nil
}

func convertOverridesToTestCases(overrides []models.TestCaseOverride) []models.ChallengeTestCase {
	var cases []models.ChallengeTestCase
	for _, ov := range overrides {
		input, _ := json.Marshal(ov.Input)
		cases = append(cases, models.ChallengeTestCase{
			Input:  json.RawMessage(input),
			Output: nil,
		})
	}
	return cases
}
