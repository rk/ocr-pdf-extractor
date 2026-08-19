package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultOllamaURL   = "http://127.0.0.1:11434"
	DefaultOllamaModel = "ministral-3:latest"
)

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	System  string                 `json:"system,omitempty"`
	Images  []string               `json:"images,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (o Options) ollamaURL() string {
	if o.OllamaURL != "" {
		return strings.TrimRight(o.OllamaURL, "/")
	}
	return DefaultOllamaURL
}

func (o Options) ollamaModel() string {
	if o.OllamaModel != "" {
		return o.OllamaModel
	}
	return DefaultOllamaModel
}

func checkOllama(opts Options) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(opts.ollamaURL() + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", opts.ollamaURL(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned HTTP %d from %s/api/tags", resp.StatusCode, opts.ollamaURL())
	}
	return nil
}

func ollamaGenerate(opts Options, model, system, prompt string, images []string, temperature float64) (string, error) {
	reqBody := ollamaGenerateRequest{
		Model:  model,
		System: system,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": temperature,
		},
	}
	if len(images) > 0 {
		reqBody.Images = images
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encoding ollama request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Post(opts.ollamaURL()+"/api/generate", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama generate HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed ollamaGenerateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("ollama: %s", parsed.Error)
	}
	return strings.TrimSpace(parsed.Response), nil
}
