package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPatchResponsesPassThroughModelForCompact(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		OriginModelName: ratio_setting.WithCompactModelSuffix("gpt-5.5"),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.5",
		},
	}

	original := []byte(`{"model":"gpt-5.5-openai-compact","input":"hello","metadata":{"trace_id":"abc"}}`)
	patched, err := patchResponsesPassThroughModelForCompact(info, original)

	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", gjson.GetBytes(patched, "model").String())
	require.Equal(t, "abc", gjson.GetBytes(patched, "metadata.trace_id").String())
}

func TestPatchResponsesPassThroughModelForCompactNoopForNormalResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "gpt-5.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.5",
		},
	}

	original := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	patched, err := patchResponsesPassThroughModelForCompact(info, original)

	require.NoError(t, err)
	require.Equal(t, string(original), string(patched))
}
