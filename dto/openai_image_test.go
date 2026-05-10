package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImageRequestPreservesJsonEditReferenceFields(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-image-1",
		"prompt": "make it sharper",
		"images": ["data:image/png;base64,aaa"],
		"mask": "data:image/png;base64,bbb",
		"input_fidelity": "high"
	}`)

	var request ImageRequest
	require.NoError(t, common.Unmarshal(raw, &request))

	encoded, err := common.Marshal(request)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "images").Exists())
	require.True(t, gjson.GetBytes(encoded, "mask").Exists())
	require.True(t, gjson.GetBytes(encoded, "input_fidelity").Exists())
	require.Equal(t, "data:image/png;base64,aaa", gjson.GetBytes(encoded, "images.0").String())
	require.Equal(t, "data:image/png;base64,bbb", gjson.GetBytes(encoded, "mask").String())
	require.Equal(t, "high", gjson.GetBytes(encoded, "input_fidelity").String())
}
