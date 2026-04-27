package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestGetEndpointTypesByChannelTypeUsesCompactEndpointForCompactAlias(t *testing.T) {
	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-5.5-openai-compact")
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponseCompact}, endpoints)
}

func TestGetEndpointTypesByChannelTypeDoesNotUseCompactEndpointForUnsupportedChannels(t *testing.T) {
	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenRouter, "gpt-5.5-openai-compact")
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, endpoints)
}
