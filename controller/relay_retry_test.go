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
		errors.New("insufficient balance, remaining quota: 4.544458"),
		types.ErrorCodeBadResponseBody,
		http.StatusForbidden,
	)

	require.True(t, shouldRetry(ctx, err, 1))
}

func TestShouldNotRetryOnLocalInsufficientQuota403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	err := types.NewErrorWithStatusCode(
		errors.New("insufficient balance, remaining quota: 4.544458"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)

	require.False(t, shouldRetry(ctx, err, 1))
}

func TestShouldNotRetryUpstreamInsufficientBalanceOnSpecificChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("specific_channel_id", "1")

	err := types.NewOpenAIError(
		errors.New("insufficient balance"),
		types.ErrorCodeBadResponseBody,
		http.StatusForbidden,
	)
	err = types.NormalizeUpstreamInsufficientBalanceError(err)

	require.False(t, shouldRetry(ctx, err, 1))
}
