package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerNormalizesUpstreamInsufficientBalance(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"Your credit balance is too low","type":"upstream_error","code":"insufficient_quota"}}`,
		)),
	}

	err := RelayErrorHandler(context.Background(), resp, true)

	require.NotNil(t, err)
	require.Equal(t, types.ErrorCodeUpstreamInsufficientBalance, err.GetErrorCode())
	require.Equal(t, types.UpstreamInsufficientBalanceMessage, err.Error())
	require.NotContains(t, err.Error(), "credit balance")
}

func TestTaskErrorFromAPIErrorMarksPreConsumeErrorsLocal(t *testing.T) {
	apiErr := types.NewErrorWithStatusCode(
		errors.New("订阅总额度不足"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)

	taskErr := TaskErrorFromAPIError(apiErr)

	require.NotNil(t, taskErr)
	require.True(t, taskErr.LocalError)
	require.Equal(t, string(types.ErrorCodeInsufficientUserQuota), taskErr.Code)
}
