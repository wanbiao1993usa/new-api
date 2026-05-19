package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryOnUpstreamInsufficientQuota403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	err := types.NewOpenAIError(
		errors.New("用户额度不足, 剩余额度: ＄-4.544458"),
		types.ErrorCodeBadResponseBody,
		http.StatusForbidden,
	)

	require.True(t, shouldRetry(ctx, err, 1))
}

func TestShouldNotRetryOnLocalInsufficientQuota403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	err := types.NewErrorWithStatusCode(
		errors.New("用户额度不足, 剩余额度: ＄-4.544458"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)

	require.False(t, shouldRetry(ctx, err, 1))
}
