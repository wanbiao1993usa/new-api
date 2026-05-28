package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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

func TestShouldNotRetryTaskLocalInsufficientQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	apiErr := types.NewErrorWithStatusCode(
		errors.New("订阅总额度不足"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
	taskErr := service.TaskErrorFromAPIError(apiErr)

	require.False(t, isTaskUpstreamInsufficientBalanceError(taskErr))
	require.False(t, shouldRetryTaskRelay(ctx, 1, taskErr, 1))
}

func TestShouldRetryTaskUpstreamInsufficientBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	taskErr := &dto.TaskError{
		Code:       "fail_to_fetch_task",
		Message:    "unexpected status 403 Forbidden: insufficient balance",
		StatusCode: http.StatusForbidden,
		Error:      errors.New("unexpected status 403 Forbidden: insufficient balance"),
	}

	require.True(t, isTaskUpstreamInsufficientBalanceError(taskErr))
	require.True(t, shouldRetryTaskRelay(ctx, 1, taskErr, 1))
}

func TestNormalizeTaskUpstreamInsufficientBalancePreservesAutoDisableSignal(t *testing.T) {
	oldAutomaticDisableChannelEnabled := common.AutomaticDisableChannelEnabled
	oldAutomaticDisableKeywords := append([]string{}, operation_setting.AutomaticDisableKeywords...)
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldAutomaticDisableChannelEnabled
		operation_setting.AutomaticDisableKeywords = oldAutomaticDisableKeywords
	})
	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableKeywords = []string{"Your credit balance is too low"}

	upstreamMessage := "Your credit balance is too low"
	taskErr := &dto.TaskError{
		Code:       "fail_to_fetch_task",
		Message:    upstreamMessage,
		StatusCode: http.StatusForbidden,
		Error:      errors.New(upstreamMessage),
	}

	autoDisableMessage := normalizeTaskUpstreamInsufficientBalanceError(taskErr)
	require.Equal(t, upstreamMessage, autoDisableMessage)
	require.Equal(t, string(types.ErrorCodeUpstreamInsufficientBalance), taskErr.Code)
	require.Equal(t, types.UpstreamInsufficientBalanceMessage, taskErr.Message)
	require.NotContains(t, taskErr.Error.Error(), "credit balance")

	err := types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
	err = types.NormalizeUpstreamInsufficientBalanceError(err)
	err.SetAutoDisableMessage(autoDisableMessage)

	require.True(t, service.ShouldDisableChannel(err))
	require.NotContains(t, err.Error(), "credit balance")
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
