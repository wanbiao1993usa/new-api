package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMessageReasoningContentPreservesExplicitEmptyString(t *testing.T) {
	raw := []byte(`{
		"role": "assistant",
		"content": "Hello",
		"reasoning_content": "",
		"reasoning": ""
	}`)

	var msg Message
	require.NoError(t, common.Unmarshal(raw, &msg))

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "reasoning_content").Exists())
	require.True(t, gjson.GetBytes(encoded, "reasoning").Exists())
	require.Equal(t, "", gjson.GetBytes(encoded, "reasoning_content").String())
	require.Equal(t, "", gjson.GetBytes(encoded, "reasoning").String())
}

func TestMessageReasoningContentOmitsAbsentField(t *testing.T) {
	raw := []byte(`{
		"role": "assistant",
		"content": "Hello"
	}`)

	var msg Message
	require.NoError(t, common.Unmarshal(raw, &msg))

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(encoded, "reasoning_content").Exists())
	require.False(t, gjson.GetBytes(encoded, "reasoning").Exists())
}

func TestMessageGetReasoningContent(t *testing.T) {
	t.Run("both nil returns empty", func(t *testing.T) {
		msg := Message{Role: "assistant"}
		require.Equal(t, "", msg.GetReasoningContent())
	})

	t.Run("ReasoningContent takes priority", func(t *testing.T) {
		rc := "thinking..."
		r := "should be ignored"
		msg := Message{ReasoningContent: &rc, Reasoning: &r}
		require.Equal(t, "thinking...", msg.GetReasoningContent())
	})

	t.Run("falls back to Reasoning when ReasoningContent is nil", func(t *testing.T) {
		r := "fallback reasoning"
		msg := Message{Reasoning: &r}
		require.Equal(t, "fallback reasoning", msg.GetReasoningContent())
	})

	t.Run("empty string value is returned", func(t *testing.T) {
		empty := ""
		msg := Message{ReasoningContent: &empty}
		require.Equal(t, "", msg.GetReasoningContent())
	})
}
