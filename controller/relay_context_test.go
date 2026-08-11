/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldReplaceFallbackWithRetryUsesStrictTokenLimit(t *testing.T) {
	belowLimit := &relaycommon.RelayInfo{}
	belowLimit.SetEstimatePromptTokens(fallbackInputTokenLimit)
	require.False(t, shouldReplaceFallbackWithRetry(belowLimit))

	overLimit := &relaycommon.RelayInfo{}
	overLimit.SetEstimatePromptTokens(fallbackInputTokenLimit + 1)
	require.True(t, shouldReplaceFallbackWithRetry(overLimit))
}

func TestNewContextTooLongErrorIsClientErrorAndSkipsRetry(t *testing.T) {
	err := newContextTooLongError()

	require.Equal(t, http.StatusBadRequest, err.StatusCode)
	require.Equal(t, types.ErrorCodeInvalidRequest, err.GetErrorCode())
	require.True(t, types.IsSkipRetryError(err))
	require.Equal(t, contextTooLongMessage, err.Error())
}

func TestFinalFallbackErrorPreservesPrimaryErrorForEmergencyChannel(t *testing.T) {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Set("emergency_used", true)
	primaryError := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusBadGateway,
	)
	fallbackError := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeDoRequestFailed,
		http.StatusGatewayTimeout,
	)

	err := finalFallbackError(ginContext, primaryError, fallbackError)

	require.Same(t, primaryError, err)
}

func TestFinalFallbackErrorMasksFallbackErrorWithoutPrimary(t *testing.T) {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	primaryError := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		http.StatusBadGateway,
	)
	fallbackError := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeDoRequestFailed,
		http.StatusGatewayTimeout,
	)

	require.Same(t, primaryError, finalFallbackError(ginContext, primaryError, fallbackError))
	err := finalFallbackError(ginContext, nil, fallbackError)
	require.NotSame(t, fallbackError, err)
	require.Equal(t, emergencyPlanFailedMessage, err.Error())
}

func TestMaskEmergencyPlanErrorMasksSingleRequestFailure(t *testing.T) {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Set("emergency_used", true)
	upstreamError := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeDoRequestFailed,
		http.StatusGatewayTimeout,
	)

	err := maskEmergencyPlanError(ginContext, upstreamError)

	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.Equal(t, emergencyPlanFailedMessage, err.Error())
	require.Equal(t, emergencyPlanFailedMessage, err.GetUserFriendlyMessage())
}

func TestMarkContextCancelledResponseUsesSuccessStatusAndKeepsDetail(t *testing.T) {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewErrorWithStatusCode(context.Canceled, types.ErrorCodeDoRequestFailed, 500)

	markContextCancelledResponse(ginContext, err)

	require.Equal(t, 200, err.StatusCode)
	require.Equal(t, requestContextCancelledMessage, err.GetUserFriendlyMessage())
	require.True(t, ginContext.GetBool("request_context_cancelled"))
	require.Equal(t, "status_code=500, context canceled", ginContext.GetString("request_context_cancelled_detail"))
}

func TestRequestContextIsCancelledIncludesDeadlineExceeded(t *testing.T) {
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancelledRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(cancelledContext)
	cancelledGin, _ := gin.CreateTestContext(httptest.NewRecorder())
	cancelledGin.Request = cancelledRequest
	cancel()
	require.True(t, requestContextIsCancelled(cancelledGin))

	deadlineContext, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer deadlineCancel()
	deadlineRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(deadlineContext)
	deadlineGin, _ := gin.CreateTestContext(httptest.NewRecorder())
	deadlineGin.Request = deadlineRequest
	require.True(t, requestContextIsCancelled(deadlineGin))
}

func TestShouldMaskQueuedRelayErrorOnlyMasksUpstreamFailures(t *testing.T) {
	require.True(t, shouldMaskQueuedRelayError(types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed)))
	require.True(t, shouldMaskQueuedRelayError(types.NewError(context.DeadlineExceeded, types.ErrorCodeBadResponseBody)))
	require.False(t, shouldMaskQueuedRelayError(types.NewError(context.Canceled, types.ErrorCodeConvertRequestFailed)))
	require.False(t, shouldMaskQueuedRelayError(types.NewError(context.Canceled, types.ErrorCodeInvalidRequest)))
}
