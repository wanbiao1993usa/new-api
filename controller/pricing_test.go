package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestFilterMarketplacePricingHidesCompactAliases(t *testing.T) {
	input := []model.Pricing{
		{ModelName: "gpt-5.5"},
		{ModelName: ratio_setting.WithCompactModelSuffix("gpt-5.5")},
		{ModelName: "claude-sonnet-4"},
	}

	filtered := filterMarketplacePricing(input)

	require.Len(t, filtered, 2)
	require.Equal(t, "gpt-5.5", filtered[0].ModelName)
	require.Equal(t, "claude-sonnet-4", filtered[1].ModelName)
}
