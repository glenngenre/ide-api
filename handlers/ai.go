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

type chatMessage struct {
	Role    string `json:"role"    example:"user"`
	Content string `json:"content" example:"Write a hello world in Go."`
}

type chatRequest struct {
	Model    string        `json:"model"    example:"llama3.2"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"   example:"false"`
}

// Chat godoc
//
//	@Summary		Chat with an Ollama model
//	@Description	Proxies a chat/completions request to the local Ollama instance. Requires a user JWT.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		chatRequest	true	"Chat request — same shape as OpenAI /v1/chat/completions"
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
	proxyToOllama(w, r, "/v1/chat/completions")
}

// Complete godoc
//
//	@Summary		Inline code completion
//	@Description	Proxies an inline completion request to Ollama. Used by the IDE's inline suggestion feature. Requires a user JWT.
//	@Tags			ai
//	@Accept			json
//	@Produce		json
//	@Param			body	body		chatRequest	true	"Completion request"
//	@Success		200		{object}	object	"Ollama response (OpenAI-compatible)"
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
	proxyToOllama(w, r, "/v1/chat/completions")
}

func proxyToOllama(w http.ResponseWriter, r *http.Request, path string) {
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

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ollamaBase+path, bytes.NewReader(body))
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

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
