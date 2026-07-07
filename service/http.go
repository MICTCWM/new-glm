package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

const (
	deferredResponseEnabledKey = "deferred_response_enabled"
	deferredResponsePayloadKey = "deferred_response_payload"
)

type deferredHTTPResponse struct {
	statusCode int
	headers    http.Header
	data       []byte
}

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. It returns false for Content-Length
// (managed separately) and X-Oneapi-Request-Id (to preserve the local instance
// ID). When the upstream header is X-Oneapi-Request-Id, the value is captured
// into the Gin context for later logging.
func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	return true
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	if shouldDeferResponse(c) {
		statusCode := http.StatusOK
		headers := make(http.Header)
		if src != nil {
			statusCode = src.StatusCode
			for k, v := range src.Header {
				if !ShouldCopyUpstreamHeader(c, k, v) {
					continue
				}
				headers[k] = append([]string(nil), v...)
			}
		}
		c.Set(deferredResponsePayloadKey, &deferredHTTPResponse{
			statusCode: statusCode,
			headers:    headers,
			data:       append([]byte(nil), data...),
		})
		return
	}

	writeHTTPResponse(c, src, data)
}

func EnableDeferredResponse(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(deferredResponseEnabledKey, true)
}

func FlushDeferredResponse(c *gin.Context) {
	if c == nil {
		return
	}
	value, ok := c.Get(deferredResponsePayloadKey)
	if !ok {
		return
	}
	payload, ok := value.(*deferredHTTPResponse)
	if !ok || payload == nil {
		return
	}
	if c.Keys != nil {
		delete(c.Keys, deferredResponsePayloadKey)
		delete(c.Keys, deferredResponseEnabledKey)
	}
	writeHTTPResponse(c, &http.Response{
		StatusCode: payload.statusCode,
		Header:     payload.headers,
	}, payload.data)
}

func shouldDeferResponse(c *gin.Context) bool {
	if c == nil {
		return false
	}
	deferEnabled, ok := c.Get(deferredResponseEnabledKey)
	return ok && deferEnabled == true
}

func writeHTTPResponse(c *gin.Context, src *http.Response, data []byte) {
	if c == nil || c.Writer == nil {
		return
	}

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		for k, v := range src.Header {
			if !ShouldCopyUpstreamHeader(c, k, v) {
				continue
			}
			c.Writer.Header().Set(k, v[0])
		}
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
