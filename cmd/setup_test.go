package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSetupConfigPreservesExistingAndConfiguresAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".maebrcode.json")
	initial := []byte(`{
  "tui": {
    "theme": "maebrcode"
  },
  "providers": {
    "old": {
      "apiKey": "keep-me"
    }
  },
  "agents": {
    "coder": {
      "maxTokens": 7000,
      "reasoningEffort": "high"
    }
  }
}`)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}

	err := writeSetupConfig(path, providerSetup{
		ProviderID:   "groq",
		ProviderType: "openai-compatible",
		APIKey:       "secret",
		BaseURL:      "https://api.groq.com/openai/v1",
		Model:        "llama-3.3-70b-versatile",
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	providers := got["providers"].(map[string]any)
	if providers["old"].(map[string]any)["apiKey"] != "keep-me" {
		t.Fatalf("existing provider was not preserved: %#v", providers["old"])
	}
	groq := providers["groq"].(map[string]any)
	if groq["type"] != "openai-compatible" {
		t.Fatalf("provider type = %v", groq["type"])
	}
	if groq["baseURL"] != "https://api.groq.com/openai/v1" {
		t.Fatalf("baseURL = %v", groq["baseURL"])
	}

	agents := got["agents"].(map[string]any)
	for _, name := range []string{"coder", "summarizer", "task", "title"} {
		agent := agents[name].(map[string]any)
		if agent["provider"] != "groq" {
			t.Fatalf("%s provider = %v", name, agent["provider"])
		}
		if agent["model"] != "llama-3.3-70b-versatile" {
			t.Fatalf("%s model = %v", name, agent["model"])
		}
		if _, ok := agent["reasoningEffort"]; ok {
			t.Fatalf("%s kept stale reasoningEffort", name)
		}
	}
	if agents["coder"].(map[string]any)["maxTokens"].(float64) != 7000 {
		t.Fatalf("coder maxTokens was not preserved")
	}
	if agents["title"].(map[string]any)["maxTokens"].(float64) != 80 {
		t.Fatalf("title maxTokens = %v", agents["title"].(map[string]any)["maxTokens"])
	}
	if got["tui"].(map[string]any)["theme"] != "maebrcode" {
		t.Fatalf("unrelated config was not preserved")
	}
}

func TestNormalizeOllamaURL(t *testing.T) {
	tests := map[string]string{
		"":                          "http://localhost:11434/v1",
		"http://localhost:11434":    "http://localhost:11434/v1",
		"http://localhost:11434/":   "http://localhost:11434/v1",
		"http://localhost:11434/v1": "http://localhost:11434/v1",
	}

	for input, want := range tests {
		if got := normalizeOllamaURL(input); got != want {
			t.Fatalf("normalizeOllamaURL(%q) = %q, want %q", input, got, want)
		}
	}
}
