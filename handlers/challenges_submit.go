package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"skwtr-ide-backend/challenges"
	"skwtr-ide-backend/db"
	"skwtr-ide-backend/middleware"
	"skwtr-ide-backend/models"
)

// SubmitChallenge godoc
//
//	@Summary		Submit a solution to a challenge
//	@Description	Submit code solution to a challenge, runs test cases via Judge0, and returns detailed results.
//	@Tags			challenges
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int	true	"Challenge ID"
//	@Param			submission	body		object	true	"Submission with language and source code"
//	@Success		200	{object}	object	"Submission results"
//	@Failure		400	{object}	errorResponse
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		502	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/challenges/{id}/submit [post]
func SubmitChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, http.StatusBadRequest, "invalid challenge id")
		return
	}
	challengeID, err := strconv.ParseInt(parts[len(parts)-2], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid challenge id")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var submitReq map[string]any
	if err := json.Unmarshal(body, &submitReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	language, ok := submitReq["language"].(string)
	if !ok || language == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}

	sourceCode, ok := submitReq["source_code"].(string)
	if !ok || sourceCode == "" {
		writeError(w, http.StatusBadRequest, "source_code is required")
		return
	}

	if !challenges.IsSupported(language) {
		writeError(w, http.StatusBadRequest, "unsupported language")
		return
	}

	var testCaseOverrides []models.TestCaseOverride
	if testCasesRaw, ok := submitReq["test_cases"].([]any); ok {
		if len(testCasesRaw) > 10 {
			writeError(w, http.StatusBadRequest, "override test cases limited to 10")
			return
		}
		for _, tc := range testCasesRaw {
			if tcMap, ok := tc.(map[string]any); ok {
				if inputRaw, ok := tcMap["input"].([]any); ok {
					testCaseOverrides = append(testCaseOverrides, models.TestCaseOverride{
						Input: inputRaw,
					})
				}
			}
		}
	}

	challenge, err := db.GetChallengeByID(challengeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load challenge")
		return
	}
	if challenge == nil {
		writeError(w, http.StatusNotFound, "challenge not found")
		return
	}

	harnessResults, judge0Result, stderrMsg, err := challenges.BuildAndTestChallenge(
		challenge,
		language,
		sourceCode,
		testCaseOverrides,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to execute submission: "+err.Error())
		return
	}

	isOverride := len(testCaseOverrides) > 0

	var expectedOutputs []json.RawMessage
	var hiddenFlags []bool

	if isOverride {
		for range testCaseOverrides {
			expectedOutputs = append(expectedOutputs, nil)
			hiddenFlags = append(hiddenFlags, false)
		}
	} else {
		for _, tc := range challenge.TestCases {
			expectedOutputs = append(expectedOutputs, tc.Output)
			hiddenFlags = append(hiddenFlags, tc.Hidden)
		}
	}

	caseResultsInterface := buildCaseResults(harnessResults, expectedOutputs, hiddenFlags, isOverride)

	passedCount := 0
	for _, r := range caseResultsInterface {
		if m, ok := r.(map[string]any); ok {
			if passed, ok := m["passed"].(bool); ok && passed {
				passedCount++
			}
		}
	}

	allPassed := passedCount == len(expectedOutputs) && !isOverride && judge0Result.Status.ID == 3

	if allPassed && !isOverride {
		userID := middleware.GetClaims(r).UserID
		_ = db.CompleteChallenge(challengeID, userID)
	}

	langCfg := challenges.GetLanguageConfig(language)
	response := map[string]any{
		"challenge_id": challengeID,
		"language":     language,
		"language_id":  langCfg.Judge0ID,
		"passed":       allPassed,
		"solved":       allPassed && !isOverride,
		"status": map[string]any{
			"id":          judge0Result.Status.ID,
			"description": judge0Result.Status.Description,
		},
		"time":         judge0Result.Time,
		"memory":       judge0Result.Memory,
		"total":        len(expectedOutputs),
		"passed_count": passedCount,
		"results":      caseResultsInterface,
	}

	if judge0Result.CompileOutput != "" {
		response["compile_output"] = judge0Result.CompileOutput
	}
	if stderrMsg != "" {
		response["stderr"] = stderrMsg
	}

	writeJSON(w, http.StatusOK, response)
}

func buildCaseResults(
	harnessResults []challenges.HarnessResult,
	expectedOutputs []json.RawMessage,
	hiddenFlags []bool,
	isOverride bool,
) []any {
	results := make([]any, 0, len(expectedOutputs))

	for i := range expectedOutputs {
		result := map[string]any{
			"index": i,
		}

		hidden := false
		if i < len(hiddenFlags) {
			hidden = hiddenFlags[i]
		}
		result["hidden"] = hidden

		var harnessResult *challenges.HarnessResult
		for j := range harnessResults {
			if harnessResults[j].Index == i {
				harnessResult = &harnessResults[j]
				break
			}
		}

		if harnessResult == nil {
			passed := false
			result["passed"] = passed
			result["error"] = "Test case not executed"
		} else if !harnessResult.Ok {
			passed := false
			result["passed"] = passed
			result["error"] = harnessResult.Err

			if !hidden {
				result["stdout"] = harnessResult.Stdout
			}
		} else {
			passed := challenges.Equal(expectedOutputs[i], harnessResult.Out)

			if isOverride && expectedOutputs[i] == nil {
				result["passed"] = nil
			} else {
				result["passed"] = passed
			}

			if !hidden {
				var actualData any
				_ = json.Unmarshal(harnessResult.Out, &actualData)
				result["actual"] = actualData

				if expectedOutputs[i] != nil {
					var expectedData any
					_ = json.Unmarshal(expectedOutputs[i], &expectedData)
					result["expected"] = expectedData
				}

				result["stdout"] = harnessResult.Stdout
			}
		}

		results = append(results, result)
	}

	return results
}
