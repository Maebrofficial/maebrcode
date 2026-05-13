package models

import "strings"

const (
	DefaultConfiguredContextWindow int64 = 128_000
	DefaultConfiguredMaxTokens     int64 = 4_096
)

func RegisterConfiguredModel(provider ModelProvider, rawModel string) Model {
	rawModel = strings.TrimSpace(rawModel)
	if rawModel == "" {
		return Model{}
	}

	if existing, ok := SupportedModels[ModelID(rawModel)]; ok {
		if existing.Provider == provider {
			return existing
		}
	}

	modelID := ModelID(string(provider) + "." + rawModel)
	if existing, ok := SupportedModels[modelID]; ok {
		return existing
	}

	model := Model{
		ID:                  modelID,
		Name:                friendlyProviderModelName(provider, rawModel),
		Provider:            provider,
		APIModel:            rawModel,
		ContextWindow:       DefaultConfiguredContextWindow,
		DefaultMaxTokens:    DefaultConfiguredMaxTokens,
		SupportsAttachments: true,
	}
	SupportedModels[model.ID] = model
	return model
}

func friendlyProviderModelName(provider ModelProvider, rawModel string) string {
	label := strings.TrimSpace(string(provider))
	if label == "" {
		return rawModel
	}
	label = strings.ToUpper(label[:1]) + label[1:]
	return label + " - " + rawModel
}
