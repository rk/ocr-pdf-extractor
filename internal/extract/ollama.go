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

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
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

func (o Options) glmOCRModel() string {
	if o.GlmOCRModel != "" {
		return o.GlmOCRModel
	}
	return DefaultGlmOCRModel
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

func checkOllamaModels(opts Options, models ...string) error {
	if err := checkOllama(opts); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(opts.ollamaURL() + "/api/tags")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading ollama tags: %w", err)
	}
	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		return fmt.Errorf("decoding ollama tags: %w", err)
	}
	available := make(map[string]bool, len(tags.Models))
	for _, m := range tags.Models {
		available[m.Name] = true
	}
	for _, want := range models {
		if available[want] {
			continue
		}
		// Ollama may list "glm-ocr:latest" while user passes "glm-ocr"
		base := strings.SplitN(want, ":", 2)[0]
		found := false
		for name := range available {
			if name == want || strings.HasPrefix(name, base+":") {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("ollama model %q not found (run: ollama pull %s)", want, want)
		}
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
