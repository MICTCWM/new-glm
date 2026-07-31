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

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
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

func TestFinalFallbackErrorMasksUpstreamErrorsForEmergencyChannel(t *testing.T) {
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

	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.Equal(t, types.ErrorCodeDoRequestFailed, err.GetErrorCode())
	require.Equal(t, emergencyPlanFailedMessage, err.Error())
	require.Equal(t, emergencyPlanFailedMessage, err.GetUserFriendlyMessage())
	require.Equal(t, emergencyPlanFailedMessage, err.ToOpenAIError().Message)
}

func TestFinalFallbackErrorPreservesOrdinaryFallbackBehavior(t *testing.T) {
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
	require.Same(t, fallbackError, finalFallbackError(ginContext, nil, fallbackError))
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

func TestDetachCancelledRequestContextForFallback(t *testing.T) {
	oldRequestMaxDuration := common.RequestMaxDuration
	common.RequestMaxDuration = 30
	t.Cleanup(func() { common.RequestMaxDuration = oldRequestMaxDuration })

	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	requestContext, cancelRequest := context.WithCancel(context.WithValue(context.Background(), "request-value", "preserved"))
	cancelRequest()
	request = request.WithContext(requestContext)
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = request

	cancelFallback := detachCancelledRequestContext(ginContext)
	require.NotNil(t, cancelFallback)
	require.NoError(t, ginContext.Request.Context().Err())
	require.Equal(t, "preserved", ginContext.Request.Context().Value("request-value"))

	cancelFallback()
	require.ErrorIs(t, ginContext.Request.Context().Err(), context.Canceled)
}

func TestDetachCancelledRequestContextLeavesLiveContextUntouched(t *testing.T) {
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = request
	originalContext := ginContext.Request.Context()

	require.Nil(t, detachCancelledRequestContext(ginContext))
	require.Equal(t, originalContext, ginContext.Request.Context())
}

func TestDetachedFallbackContextAllowsResponseWrites(t *testing.T) {
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	requestContext, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	request = request.WithContext(requestContext)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = request

	// Before detaching, the same write path used by streaming error responses
	// must reject the cancelled request context.
	require.ErrorIs(t, helper.FlushWriter(ginContext), context.Canceled)

	cancelFallback := detachCancelledRequestContext(ginContext)
	require.NotNil(t, cancelFallback)
	t.Cleanup(cancelFallback)

	// The fallback response can now be flushed instead of becoming a second
	// "request context done: context canceled" failure.
	require.NoError(t, helper.FlushWriter(ginContext))
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
