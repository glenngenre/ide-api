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

	caseResults, err := challenges.BuildAndTest(
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

	hiddenFlags := make([]bool, len(caseResults))
	if !isOverride {
		for i, tc := range challenge.TestCases {
			hiddenFlags[i] = tc.Hidden
		}
	}

	caseResultsInterface := buildCaseResults(caseResults, hiddenFlags, isOverride)

	passedCount := 0
	status := models.CodeStatus{ID: 3, Description: "Accepted"}
	var totalTime float64
	var maxMemory int
	var compileOutputs, stderrMessages []string
	for i, result := range caseResults {
		executionFailed := result.Error != ""
		if !executionFailed && result.Passed && !isOverride {
			passedCount++
		}
		if executionFailed && status.ID == 3 {
			status = models.CodeStatus{ID: result.StatusID, Description: result.Status}
			if status.ID == 3 {
				status = models.CodeStatus{ID: 13, Description: "Internal Error"}
			}
		}
		if seconds, err := strconv.ParseFloat(result.Time, 64); err == nil {
			totalTime += seconds
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}
		if !hiddenFlags[i] {
			if result.CompileOutput != "" {
				compileOutputs = append(compileOutputs, result.CompileOutput)
			}
			if result.Stderr != "" {
				stderrMessages = append(stderrMessages, result.Stderr)
			}
		}
	}

	allPassed := len(caseResults) > 0 && passedCount == len(caseResults) && !isOverride && status.ID == 3

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
			"id":          status.ID,
			"description": status.Description,
		},
		"time":         strconv.FormatFloat(totalTime, 'f', -1, 64),
		"memory":       maxMemory,
		"total":        len(caseResults),
		"passed_count": passedCount,
		"results":      caseResultsInterface,
	}

	if len(compileOutputs) > 0 {
		response["compile_output"] = strings.Join(compileOutputs, "\n")
	}
	if len(stderrMessages) > 0 {
		response["stderr"] = strings.Join(stderrMessages, "\n")
	}

	writeJSON(w, http.StatusOK, response)
}

func buildCaseResults(
	caseResults []challenges.CaseResult,
	hiddenFlags []bool,
	isOverride bool,
) []any {
	results := make([]any, 0, len(caseResults))

	for i, caseResult := range caseResults {
		hidden := i < len(hiddenFlags) && hiddenFlags[i]
		executionFailed := caseResult.Error != ""
		result := map[string]any{
			"index":  caseResult.Index,
			"hidden": hidden,
			"passed": caseResult.Passed && !executionFailed,
			"status": map[string]any{
				"id":          caseResult.StatusID,
				"description": caseResult.Status,
			},
			"time":   caseResult.Time,
			"memory": caseResult.Memory,
		}

		if executionFailed {
			result["error"] = "Test case execution failed"
			if !hidden && caseResult.Error != "" {
				result["error"] = caseResult.Error
			}
		} else if isOverride {
			result["passed"] = nil
		}

		if !hidden {
			result["actual"] = caseResult.Out
			result["out"] = caseResult.Out
			result["stdout"] = caseResult.UserOut
			result["user_out"] = caseResult.UserOut
			if !executionFailed && !isOverride && caseResult.Expected != nil {
				result["expected"] = caseResult.Expected
			}
			if caseResult.Stderr != "" {
				result["stderr"] = caseResult.Stderr
			}
			if caseResult.CompileOutput != "" {
				result["compile_output"] = caseResult.CompileOutput
			}
		}

		results = append(results, result)
	}

	return results
}
