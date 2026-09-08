package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

var ollamaBase string

func init() {
	ollamaBase = os.Getenv("OLLAMA_BASE_URL")
	if ollamaBase == "" {
		ollamaBase = "http://ollama:11434"
	}
}

type errorResponse struct {
	Error string `json:"error" example:"something went wrong"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// Chat godoc
//
//	@Summary		Chat with an Ollama model
//	@Description	Proxies a chat/completions request to Ollama's /v1/chat/completions.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		chatRequest	true	"Chat request (OpenAI format)"
//	@Success		200		{object}	object	"Ollama response (OpenAI-compatible)"
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		502		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/ai/chat [post]
func Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	proxyToOllamaChat(w, r)
}

func proxyToOllamaChat(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ollamaBase+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create proxy request")
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	resp, err := (&http.Client{}).Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach Ollama")
		return
	}
	defer resp.Body.Close()

	copyResponse(w, resp)
}

// Complete godoc
//
//	@Summary		Inline code completion
//	@Description	Proxies a completion request to Ollama's /api/generate.
//	@Description	Accepts { model, prompt, suffix, stream, options }.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object	true	"Completion request"
//	@Success		200		{object}	object	"Ollama response (native format: { response })"
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		502		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/v1/ai/complete [post]
func Complete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var temp struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &temp); err != nil || temp.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ollamaBase+"/api/generate", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create proxy request")
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	if r.Header.Get("Accept") == "text/event-stream" {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	resp, err := (&http.Client{}).Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach Ollama")
		return
	}
	defer resp.Body.Close()

	copyResponse(w, resp)
}

func copyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
