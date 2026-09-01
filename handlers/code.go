package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"skwtr-ide-backend/models"
)

var judge0Base string

func init() {
	judge0Base = strings.TrimRight(os.Getenv("JUDGE0_BASE_URL"), "/")
	if judge0Base == "" {
		judge0Base = "https://judge0.apps.skwtr.com"
	}
}

func applyJudge0Headers(req *http.Request) {
	if auth := strings.TrimSpace(os.Getenv("JUDGE0_AUTHN_TOKEN")); auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			req.Header.Set("Authorization", auth)
		} else {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
	}
}

// RunCode godoc
//
//	@Summary		Submit code to Judge0
//	@Description	Forwards a Judge0-style submission request to the remote judge service and returns the queued result.
//	@Tags			code
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.CodeSubmission		true	"Judge0 submission payload"
//	@Success		200		{object}	models.CodeSubmissionResult	"Judge0 submission token"
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		502		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/code/run [post]
func RunCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeError(w, http.StatusBadRequest, "request body is required")
		return
	}

	var submission models.CodeSubmission
	if err := json.Unmarshal(body, &submission); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if submission.SourceCode == "" || submission.LanguageID == 0 {
		writeError(w, http.StatusBadRequest, "source_code and language_id are required")
		return
	}

	proxyBody, err := json.Marshal(submission)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode submission")
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, judge0Base+"/submissions?base64_encoded=true&wait=false", bytes.NewReader(proxyBody))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create proxy request")
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", "application/json")
	applyJudge0Headers(proxyReq)

	resp, err := (&http.Client{}).Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach judge0")
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// GetSubmissionStatus godoc
//
//	@Summary		Fetch Judge0 submission status
//	@Description	Poll the Judge0 submission status for a previously created token.
//	@Tags			code
//	@Produce		json
//	@Param			token	path		string						true	"Submission token"
//	@Success		200		{object}	models.CodeSubmissionResult	"Judge0 submission status"
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		502		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/code/status/{token} [get]
func GetSubmissionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/code/status/"), "/")
	if token == "" {
		writeError(w, http.StatusBadRequest, "submission token is required")
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, judge0Base+"/submissions/"+token+"?base64_encoded=true", nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create proxy request")
		return
	}
	proxyReq.Header.Set("Accept", "application/json")
	applyJudge0Headers(proxyReq)

	resp, err := (&http.Client{}).Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach judge0")
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
