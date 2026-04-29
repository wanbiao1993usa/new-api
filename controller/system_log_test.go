package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSystemLogTail(t *testing.T) {
	assert.Equal(t, defaultSystemLogTailLines, parseSystemLogTail(""))
	assert.Equal(t, defaultSystemLogTailLines, parseSystemLogTail("-1"))
	assert.Equal(t, 42, parseSystemLogTail("42"))
	assert.Equal(t, maxSystemLogTailLines, parseSystemLogTail("999999"))
}

func TestReadSystemLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oneapi-test.log")
	content := "line-1\nline-2\nline-3\nline-4\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	lines, truncated, readLimitHit, err := readSystemLogTail(path, 2)

	require.NoError(t, err)
	assert.True(t, truncated)
	assert.False(t, readLimitHit)
	assert.Equal(t, []string{"line-3", "line-4"}, lines)
}

func TestFilterSystemLogLines(t *testing.T) {
	lines := []string{
		"[INFO] request success",
		"[ERR] failed to settle billing",
		"[WARN] retry upstream",
	}

	filtered := filterSystemLogLines(lines, "err")

	assert.Equal(t, []string{"[ERR] failed to settle billing"}, filtered)
}
