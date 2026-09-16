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
	SourceCode    string `json:"source_code"`
	LanguageID    int    `json:"language_id"`
	Stdin         string `json:"stdin"`
	CPUTimeLimit  int    `json:"cpu_time_limit"`
	WallTimeLimit int    `json:"wall_time_limit"`
	MemoryLimit   int    `json:"memory_limit"`
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
	Index   int             `json:"-"`
	Status  string          `json:"status"`
	Value   json.RawMessage `json:"value,omitempty"`
	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
	Trace   string          `json:"trace,omitempty"`
}

var judge0Base string

func init() {
	judge0Base = strings.TrimRight(os.Getenv("JUDGE0_BASE_URL"), "/")
	if judge0Base == "" {
		judge0Base = "https://judge0.apps.skwtr.com"
	}
}

func SubmitToJudge0(languageID int, sourceCode string, stdin string, cpuTimeLimit int, memoryLimit int) (*judge0Response, error) {
	submission := judge0Submission{
		SourceCode:    base64.StdEncoding.EncodeToString([]byte(sourceCode)),
		LanguageID:    languageID,
		Stdin:         base64.StdEncoding.EncodeToString([]byte(stdin)),
		CPUTimeLimit:  cpuTimeLimit,
		WallTimeLimit: cpuTimeLimit + 5,
		MemoryLimit:   memoryLimit,
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

		var result judge0Response
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			attempt++
			continue
		}

		if result.Status.ID == 1 || result.Status.ID == 2 {
			attempt++
			continue
		}

		result.Stdout = decodeBase64(result.Stdout)
		result.Stderr = decodeBase64(result.Stderr)
		result.CompileOutput = decodeBase64(result.CompileOutput)

		return &result, nil
	}

	return nil, fmt.Errorf("polling timeout after %d attempts", maxAttempts)
}

func ParseHarnessResult(stdout string) (*HarnessResult, error) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimPrefix(line, "\x1e")
		if !strings.HasPrefix(line, "__SKWTR__ ") {
			continue
		}
		var result HarnessResult
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "__SKWTR__ ")), &result); err != nil {
			return nil, fmt.Errorf("harness marker found but JSON invalid: %w", err)
		}
		if result.Status == "" {
			return nil, fmt.Errorf("harness marker present but status missing")
		}
		return &result, nil
	}
	return nil, fmt.Errorf("no harness marker in stdout")
}

func BuildAndTestChallenge(
	challenge *models.Challenge,
	language string,
	sourceCode string,
	testCaseOverrides []models.TestCaseOverride,
) ([]HarnessResult, *judge0Response, string, error) {
	return buildAndTestChallenge(challenge, language, sourceCode, testCaseOverrides, nil)
}

func BuildAndTestChallengeStream(
	challenge *models.Challenge,
	language string,
	sourceCode string,
	testCaseOverrides []models.TestCaseOverride,
	onResult func(result HarnessResult, expected json.RawMessage, hidden bool, isOverride bool),
) ([]HarnessResult, *judge0Response, string, error) {
	return buildAndTestChallenge(challenge, language, sourceCode, testCaseOverrides, onResult)
}

func buildAndTestChallenge(
	challenge *models.Challenge,
	language string,
	sourceCode string,
	testCaseOverrides []models.TestCaseOverride,
	onResult func(result HarnessResult, expected json.RawMessage, hidden bool, isOverride bool),
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

	fullSource, err := langCfg.Builder.Build(challenge.FunctionName, sourceCode)
	if err != nil {
		return nil, nil, "", fmt.Errorf("build harness: %w", err)
	}

	var results []HarnessResult
	var judge0Result *judge0Response
	var stderrMessages []string
	for i, tc := range testCases {
		var args []any
		if err := json.Unmarshal(tc.Input, &args); err != nil {
			return nil, judge0Result, "", fmt.Errorf("case %d: bad input: %w", i, err)
		}
		stdin, err := json.Marshal(args)
		if err != nil {
			return nil, judge0Result, "", fmt.Errorf("case %d: marshal input: %w", i, err)
		}

		resp, submitErr := SubmitToJudge0(langCfg.Judge0ID, fullSource, string(stdin), 5, 256000)
		if submitErr != nil {
			result := HarnessResult{Index: i, Status: "internal_error", Message: submitErr.Error()}
			results = append(results, result)
			emitHarnessResult(onResult, result, tc.Output, tc.Hidden, len(testCaseOverrides) > 0)
			continue
		}
		judge0Result = resp

		result, parseErr := ParseHarnessResult(resp.Stdout)
		if parseErr == nil {
			result.Index = i
			results = append(results, *result)
			emitHarnessResult(onResult, *result, tc.Output, tc.Hidden, len(testCaseOverrides) > 0)
			continue
		}

		classified := classifyJudge0Result(i, resp)
		results = append(results, classified)
		emitHarnessResult(onResult, classified, tc.Output, tc.Hidden, len(testCaseOverrides) > 0)
		if message := firstNonEmpty(resp.Stderr, resp.CompileOutput, resp.Stdout); message != "" {
			stderrMessages = append(stderrMessages, message)
		}
	}

	if judge0Result == nil {
		judge0Result = &judge0Response{}
	}
	return results, judge0Result, strings.Join(stderrMessages, "\n"), nil
}

func emitHarnessResult(
	onResult func(result HarnessResult, expected json.RawMessage, hidden bool, isOverride bool),
	result HarnessResult,
	expected json.RawMessage,
	hidden bool,
	isOverride bool,
) {
	if onResult != nil {
		onResult(result, expected, hidden, isOverride)
	}
}

func classifyJudge0Result(index int, resp *judge0Response) HarnessResult {
	switch resp.Status.ID {
	case 6:
		return HarnessResult{Index: index, Status: "compile_error", Message: firstNonEmpty(resp.CompileOutput, resp.Stderr)}
	case 5:
		return HarnessResult{Index: index, Status: "time_limit", Message: "time limit exceeded"}
	case 7, 8, 9, 10, 11, 12:
		return HarnessResult{Index: index, Status: "runtime_error", Message: firstNonEmpty(resp.Stderr, resp.CompileOutput)}
	default:
		return HarnessResult{Index: index, Status: "internal_error", Message: firstNonEmpty(resp.Stderr, resp.CompileOutput, resp.Stdout)}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func decodeBase64(value string) string {
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return value
	}
	return string(decoded)
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
