package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mohammadtihame/maebrcode/internal/config"
	"github.com/mohammadtihame/maebrcode/internal/llm/models"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type setupProvider struct {
	Name         string
	Description  string
	ID           string
	Type         string
	BaseURL      string
	APIKeyEnv    string
	DefaultModel string
	NeedsAPIKey  bool
}

type providerSetup struct {
	ProviderID   string
	ProviderType string
	APIKey       string
	BaseURL      string
	Model        string
}

var errSetupCanceled = errors.New("setup canceled")

var setupProviders = []setupProvider{
	{
		Name:         "OpenAI",
		Description:  "Use this if you have an OpenAI API key.",
		ID:           string(models.ProviderOpenAI),
		Type:         string(models.ProviderOpenAI),
		APIKeyEnv:    "OPENAI_API_KEY",
		DefaultModel: string(models.GPT41),
		NeedsAPIKey:  true,
	},
	{
		Name:         "Anthropic",
		Description:  "Use this if you have an Anthropic Claude API key.",
		ID:           string(models.ProviderAnthropic),
		Type:         string(models.ProviderAnthropic),
		APIKeyEnv:    "ANTHROPIC_API_KEY",
		DefaultModel: string(models.Claude4Sonnet),
		NeedsAPIKey:  true,
	},
	{
		Name:         "Google Gemini",
		Description:  "Use this if you have a Gemini API key.",
		ID:           string(models.ProviderGemini),
		Type:         string(models.ProviderGemini),
		APIKeyEnv:    "GEMINI_API_KEY",
		DefaultModel: string(models.Gemini25),
		NeedsAPIKey:  true,
	},
	{
		Name:         "Groq",
		Description:  "Fast OpenAI-compatible hosted models.",
		ID:           "groq",
		Type:         string(models.ProviderOpenAICompatible),
		BaseURL:      "https://api.groq.com/openai/v1",
		APIKeyEnv:    "GROQ_API_KEY",
		DefaultModel: "llama-3.3-70b-versatile",
		NeedsAPIKey:  true,
	},
	{
		Name:         "OpenRouter",
		Description:  "Use many hosted models with one OpenRouter key.",
		ID:           string(models.ProviderOpenRouter),
		Type:         string(models.ProviderOpenRouter),
		APIKeyEnv:    "OPENROUTER_API_KEY",
		DefaultModel: string(models.OpenRouterClaude37Sonnet),
		NeedsAPIKey:  true,
	},
	{
		Name:         "xAI",
		Description:  "Use this if you have an xAI API key.",
		ID:           string(models.ProviderXAI),
		Type:         string(models.ProviderXAI),
		APIKeyEnv:    "XAI_API_KEY",
		DefaultModel: string(models.XAIGrok3Beta),
		NeedsAPIKey:  true,
	},
	{
		Name:         "Ollama",
		Description:  "Run a local model on your computer.",
		ID:           string(models.ProviderOllama),
		Type:         string(models.ProviderOllama),
		BaseURL:      "http://localhost:11434/v1",
		DefaultModel: "llama3.2",
		NeedsAPIKey:  false,
	},
	{
		Name:         "Custom OpenAI-compatible API",
		Description:  "Use a provider with an OpenAI-style /chat/completions API.",
		ID:           "custom",
		Type:         string(models.ProviderOpenAICompatible),
		DefaultModel: "model-name",
		NeedsAPIKey:  true,
	},
}

func newSetupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Set up MaebrCode with an AI provider",
		Long:  "Set up MaebrCode with an AI provider using a simple guided terminal wizard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runSetupWizard(os.Stdin, cmd.OutOrStdout())
			if errors.Is(err, errSetupCanceled) {
				return nil
			}
			return err
		},
	}
}

func loadConfigOrOfferSetup(cwd string, debug bool, allowPrompt bool) (*config.Config, error) {
	cfg, err := config.Load(cwd, debug)
	if err == nil {
		return cfg, nil
	}
	if !config.IsProviderSetupError(err) {
		return nil, err
	}
	if !allowPrompt || !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, setupNeededError()
	}

	fmt.Fprintln(os.Stderr, "MaebrCode is not connected to an AI provider yet.")
	fmt.Fprintln(os.Stderr, "I can help set that up now.")
	reader := bufio.NewReader(os.Stdin)
	ok, readErr := askYesNo(reader, os.Stderr, "Start setup now?", true)
	if readErr != nil {
		return nil, readErr
	}
	if !ok {
		return nil, setupNeededError()
	}

	if err := runSetupWizardWithReader(os.Stdin, reader, os.Stderr); err != nil {
		if errors.Is(err, errSetupCanceled) {
			return nil, setupNeededError()
		}
		return nil, err
	}

	config.Reset()
	cfg, err = config.Load(cwd, debug)
	if err != nil {
		return nil, fmt.Errorf("setup was saved, but MaebrCode still could not start: %w", err)
	}
	return cfg, nil
}

func setupNeededError() error {
	return errors.New("MaebrCode needs an AI provider before it can run.\n\nRun `maebrcode setup` and choose OpenAI, Anthropic, Gemini, Groq, Ollama, or a custom OpenAI-compatible API.")
}

func startupNote(enabled bool, message string) {
	if !enabled || !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	fmt.Fprintf(os.Stderr, "MaebrCode: %s...\n", message)
}

func runSetupWizard(in *os.File, out io.Writer) error {
	return runSetupWizardWithReader(in, bufio.NewReader(in), out)
}

func runSetupWizardWithReader(in *os.File, reader *bufio.Reader, out io.Writer) error {
	fmt.Fprintln(out, "MaebrCode setup")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Choose where MaebrCode should send AI requests.")
	fmt.Fprintln(out, "You can change this later by running `maebrcode setup` again.")
	fmt.Fprintln(out, "")
	for i, provider := range setupProviders {
		fmt.Fprintf(out, "  %d) %s - %s\n", i+1, provider.Name, provider.Description)
	}
	fmt.Fprintln(out, "  0) Cancel")
	fmt.Fprintln(out, "")

	provider, err := askProvider(reader, out)
	if err != nil {
		return err
	}
	if provider == nil {
		fmt.Fprintln(out, "Setup canceled.")
		return errSetupCanceled
	}

	setup := providerSetup{
		ProviderID:   provider.ID,
		ProviderType: provider.Type,
		BaseURL:      provider.BaseURL,
	}

	if provider.Name == "Custom OpenAI-compatible API" {
		id, err := askLine(reader, out, "Provider name", provider.ID)
		if err != nil {
			return err
		}
		id = sanitizeProviderID(id)
		if id == "" {
			return fmt.Errorf("provider name is required")
		}
		setup.ProviderID = id

		baseURL, err := askRequiredLine(reader, out, "Base URL, for example https://api.example.com/v1")
		if err != nil {
			return err
		}
		setup.BaseURL = strings.TrimRight(baseURL, "/")
	}

	if provider.Type == string(models.ProviderOllama) {
		baseURL, err := askLine(reader, out, "Ollama URL", setup.BaseURL)
		if err != nil {
			return err
		}
		setup.BaseURL = normalizeOllamaURL(baseURL)
		setup.APIKey = "ollama"
	} else if provider.NeedsAPIKey {
		apiKey, err := askAPIKey(in, reader, out, provider)
		if err != nil {
			return err
		}
		setup.APIKey = apiKey
	}

	model, err := askLine(reader, out, "Model", provider.DefaultModel)
	if err != nil {
		return err
	}
	model = strings.TrimSpace(model)
	if model == "" || model == "model-name" {
		return fmt.Errorf("model is required")
	}
	setup.Model = model

	configPath, err := defaultSetupConfigPath()
	if err != nil {
		return err
	}
	if err := writeSetupConfig(configPath, setup); err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Done. Saved setup to %s\n", configPath)
	fmt.Fprintln(out, "Run `maebrcode` to start.")
	return nil
}

func askProvider(reader *bufio.Reader, out io.Writer) (*setupProvider, error) {
	for {
		answer, err := askLine(reader, out, "Provider", "1")
		if err != nil {
			return nil, err
		}
		answer = strings.TrimSpace(answer)
		if answer == "0" {
			return nil, nil
		}
		idx, err := strconv.Atoi(answer)
		if err != nil || idx < 1 || idx > len(setupProviders) {
			fmt.Fprintf(out, "Please enter a number from 0 to %d.\n", len(setupProviders))
			continue
		}
		return &setupProviders[idx-1], nil
	}
}

func askAPIKey(in *os.File, reader *bufio.Reader, out io.Writer, provider *setupProvider) (string, error) {
	if provider.APIKeyEnv != "" {
		if key := strings.TrimSpace(os.Getenv(provider.APIKeyEnv)); key != "" {
			useEnv, err := askYesNo(reader, out, fmt.Sprintf("Use %s from your environment?", provider.APIKeyEnv), true)
			if err != nil {
				return "", err
			}
			if useEnv {
				return key, nil
			}
		}
	}

	for {
		fmt.Fprint(out, "API key: ")
		var key string
		if term.IsTerminal(int(in.Fd())) {
			bytes, err := term.ReadPassword(int(in.Fd()))
			fmt.Fprintln(out)
			if err != nil {
				return "", err
			}
			key = string(bytes)
		} else {
			line, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return "", err
			}
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				return "", fmt.Errorf("API key is required for %s", provider.Name)
			}
			key = line
		}
		key = strings.TrimSpace(key)
		if key != "" {
			return key, nil
		}
		fmt.Fprintln(out, "API key is required for this provider.")
	}
}

func askRequiredLine(reader *bufio.Reader, out io.Writer, prompt string) (string, error) {
	for {
		value, err := askLine(reader, out, prompt, "")
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value, nil
		}
		fmt.Fprintln(out, "This value is required.")
	}
}

func askLine(reader *bufio.Reader, out io.Writer, prompt string, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", prompt)
	}
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultValue, nil
	}
	return answer, nil
}

func askYesNo(reader *bufio.Reader, out io.Writer, prompt string, defaultYes bool) (bool, error) {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}
	for {
		answer, err := askLine(reader, out, prompt+" "+suffix, "")
		if err != nil {
			return false, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "" {
			return defaultYes, nil
		}
		switch answer {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(out, "Please answer yes or no.")
		}
	}
}

func defaultSetupConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find your home directory: %w", err)
	}
	return filepath.Join(home, ".maebrcode.json"), nil
}

func writeSetupConfig(path string, setup providerSetup) error {
	if setup.ProviderID == "" {
		return fmt.Errorf("provider id is required")
	}
	if setup.ProviderType == "" {
		setup.ProviderType = setup.ProviderID
	}
	if setup.Model == "" {
		return fmt.Errorf("model is required")
	}
	if setup.ProviderType == string(models.ProviderOpenAICompatible) && setup.BaseURL == "" {
		return fmt.Errorf("base URL is required for OpenAI-compatible providers")
	}

	cfg, err := readConfigMap(path)
	if err != nil {
		return err
	}

	providers := objectValue(cfg, "providers")
	providerCfg := objectValue(providers, setup.ProviderID)
	providerCfg["type"] = setup.ProviderType
	providerCfg["disabled"] = false
	if setup.APIKey != "" {
		providerCfg["apiKey"] = setup.APIKey
	}
	if setup.BaseURL != "" {
		providerCfg["baseURL"] = setup.BaseURL
	} else {
		delete(providerCfg, "baseURL")
	}

	agents := objectValue(cfg, "agents")
	for _, name := range []string{"coder", "summarizer", "task", "title"} {
		agentCfg := objectValue(agents, name)
		agentCfg["provider"] = setup.ProviderID
		agentCfg["model"] = setup.Model
		if name == "title" {
			agentCfg["maxTokens"] = 80
		} else if _, ok := agentCfg["maxTokens"]; !ok {
			agentCfg["maxTokens"] = 5000
		}
		delete(agentCfg, "reasoningEffort")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func readConfigMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

func objectValue(parent map[string]any, key string) map[string]any {
	if value, ok := parent[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	parent[key] = value
	return value
}

func normalizeOllamaURL(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		url = "http://localhost:11434"
	}
	if strings.HasSuffix(url, "/v1") {
		return url
	}
	return url + "/v1"
}

func sanitizeProviderID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, " ", "-")
	return id
}
