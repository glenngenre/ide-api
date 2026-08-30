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

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	proxyToOllama(w, r, "/v1/chat/completions")
}

func Complete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	proxyToOllama(w, r, "/v1/chat/completions")
}

func proxyToOllama(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		http.Error(w, `{"error":"model is required"}`, http.StatusBadRequest)
		return
	}

	target := ollamaBase + path

	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":"failed to create proxy request"}`, http.StatusInternalServerError)
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")

	if req.Stream {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, `{"error":"failed to reach Ollama"}`, http.StatusBadGateway)
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
