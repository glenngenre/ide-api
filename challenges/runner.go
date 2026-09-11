package challenges

import (
	"encoding/json"
	"fmt"
	"strings"

	"skwtr-ide-backend/challenges/harness"
	"skwtr-ide-backend/models"
)

type CaseResult struct {
	Index         int             `json:"index"`
	Passed        bool            `json:"passed"`
	Stdout        string          `json:"stdout"`
	UserOut       string          `json:"user_out"`
	Stderr        string          `json:"stderr"`
	Out           json.RawMessage `json:"out"`
	Expected      json.RawMessage `json:"expected"`
	Status        string          `json:"status"`
	StatusID      int             `json:"status_id"`
	CompileOutput string          `json:"compile_output,omitempty"`
	Error         string          `json:"error,omitempty"`
	Time          string          `json:"time"`
	Memory        int             `json:"memory"`
}

func BuildAndTest(
	challenge *models.Challenge,
	language string,
	sourceCode string,
	overrides []models.TestCaseOverride,
) ([]CaseResult, error) {
	langCfg := GetLanguageConfig(language)
	if langCfg == nil {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	testCases := challenge.TestCases
	if len(overrides) > 0 {
		var err error
		testCases, err = convertOverridesToTestCases(overrides)
		if err != nil {
			return nil, err
		}
	}

	wrappedSource := langCfg.Builder.WrapWithStdin(challenge.FunctionName, sourceCode)

	stdinCases := make([]string, len(testCases))
	for i, tc := range testCases {
		var args []json.RawMessage
		if err := json.Unmarshal(tc.Input, &args); err != nil || args == nil {
			return nil, fmt.Errorf("test case %d input must be a JSON array", i)
		}
		stdinCases[i] = string(tc.Input)
	}

	judge0Results, err := SubmitBatch(langCfg.Judge0ID, wrappedSource, stdinCases)
	if err != nil {
		return nil, fmt.Errorf("batch submit: %w", err)
	}
	if len(judge0Results) != len(testCases) {
		return nil, fmt.Errorf("judge0 returned %d results for %d test cases", len(judge0Results), len(testCases))
	}

	results := make([]CaseResult, len(testCases))
	for i, jr := range judge0Results {
		userOut, resultLine := splitStdout(jr.Stdout)
		expected := testCases[i].Output

		var out json.RawMessage
		var executionError string
		if jr.Status.ID != 3 {
			executionError = jr.CompileOutput
			if executionError == "" {
				executionError = jr.Stderr
			}
			if executionError == "" {
				executionError = jr.Status.Description
			}
			if executionError == "" {
				executionError = fmt.Sprintf("Execution failed with status %d", jr.Status.ID)
			}
		} else if resultLine == "" || !json.Valid([]byte(resultLine)) {
			executionError = "Missing or invalid JSON result from harness"
		} else {
			out = json.RawMessage(resultLine)
		}

		results[i] = CaseResult{
			Index:         i,
			Passed:        executionError == "" && expected != nil && jsonEqual(out, expected),
			Stdout:        jr.Stdout,
			UserOut:       userOut,
			Stderr:        jr.Stderr,
			Out:           out,
			Expected:      expected,
			Status:        jr.Status.Description,
			StatusID:      jr.Status.ID,
			CompileOutput: jr.CompileOutput,
			Error:         executionError,
			Time:          jr.Time,
			Memory:        jr.Memory,
		}
	}

	return results, nil
}

func splitStdout(raw string) (userOut string, resultLine string) {
	for offset := 0; offset < len(raw); {
		relative := strings.Index(raw[offset:], harness.ResultPrefix)
		if relative < 0 {
			break
		}
		start := offset + relative
		payloadStart := start + len(harness.ResultPrefix)
		lineEnd := strings.IndexByte(raw[payloadStart:], '\n')
		if lineEnd < 0 {
			break;
		}
		end := payloadStart + lineEnd
		payload := strings.TrimSuffix(raw[payloadStart:end], "\r")
		if json.Valid([]byte(payload)) {
			return raw[:start] + raw[end+1:], payload
		}
		offset = payloadStart
	}
	return raw, ""
}

// jsonEqual compares two JSON values semantically.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	aj, _ := json.Marshal(av)
	bj, _ := json.Marshal(bv)
	return string(aj) == string(bj)
}

func convertOverridesToTestCases(overrides []models.TestCaseOverride) ([]models.ChallengeTestCase, error) {
	cases := make([]models.ChallengeTestCase, len(overrides))
	for i, ov := range overrides {
		input, err := json.Marshal(ov.Input)
		if err != nil {
			return nil, fmt.Errorf("marshal test case %d input: %w", i, err)
		}
		cases[i] = models.ChallengeTestCase{Input: input}
	}
	return cases, nil
}
