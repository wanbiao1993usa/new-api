package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateImageModelForImageEndpointRejectsTextModels(t *testing.T) {
	for _, modelName := range []string{
		"gpt-5.5",
		"gpt5.5",
		"gpt-5-mini",
		"gpt5-mini",
		"GPT-4O",
		"o3",
		"chatgpt-4o-latest",
	} {
		t.Run(modelName, func(t *testing.T) {
			require.Error(t, validateImageModelForImageEndpoint(modelName))
		})
	}
}

func TestValidateImageModelForImageEndpointAllowsImageModelsAndAliases(t *testing.T) {
	for _, modelName := range []string{
		"gpt-image-1",
		"gpt-image-2",
		"dall-e-3",
		"imagen-4.0-generate-001",
		"custom-image-alias",
		"",
	} {
		t.Run(modelName, func(t *testing.T) {
			require.NoError(t, validateImageModelForImageEndpoint(modelName))
		})
	}
}
