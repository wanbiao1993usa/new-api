package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizeImageUsageForBillingUsesEstimateForRatioModel(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(12)
	request := &dto.ImageRequest{
		Model:  "gpt-5.5",
		Prompt: "draw a poster",
		Size:   "1024x1024",
	}
	usage := &dto.Usage{}

	normalizeImageUsageForBilling(info, request, usage)

	require.Equal(t, 12, usage.PromptTokens)
	require.Equal(t, 1584, usage.CompletionTokens)
	require.Equal(t, 1596, usage.TotalTokens)
	require.Equal(t, 12, usage.PromptTokensDetails.TextTokens)
	require.Equal(t, 1584, usage.CompletionTokenDetails.ImageTokens)
}

func TestNormalizeImageUsageForBillingKeepsPriceModelBillable(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{UsePrice: true},
	}
	usage := &dto.Usage{}

	normalizeImageUsageForBilling(info, &dto.ImageRequest{}, usage)

	require.Equal(t, 1, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 1, usage.TotalTokens)
}

func TestNormalizeImageUsageForBillingMapsTotalOnlyUsageToImageOutput(t *testing.T) {
	usage := &dto.Usage{TotalTokens: 200}

	normalizeImageUsageForBilling(nil, nil, usage)

	require.Equal(t, 0, usage.PromptTokens)
	require.Equal(t, 200, usage.CompletionTokens)
	require.Equal(t, 200, usage.TotalTokens)
	require.Equal(t, 200, usage.CompletionTokenDetails.ImageTokens)
}
