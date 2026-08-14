package common

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

// LimitedCaptureBuffer keeps request/response diagnostics bounded. Relay
// responses can be unbounded SSE streams, so diagnostic capture must never be
// allowed to retain the whole stream in memory.
type LimitedCaptureBuffer struct {
	buf       bytes.Buffer
	maxBytes  int
	truncated bool
}

func NewLimitedCaptureBuffer(maxBytes int) *LimitedCaptureBuffer {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &LimitedCaptureBuffer{maxBytes: maxBytes}
}

func (b *LimitedCaptureBuffer) Write(p []byte) (int, error) {
	if b.maxBytes == 0 {
		b.truncated = b.truncated || len(p) > 0
		return len(p), nil
	}
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || len(p) > 0
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *LimitedCaptureBuffer) WriteString(s string) (int, error) {
	if b.maxBytes == 0 {
		b.truncated = b.truncated || len(s) > 0
		return len(s), nil
	}
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || len(s) > 0
		return len(s), nil
	}
	if len(s) > remaining {
		_, _ = b.buf.WriteString(s[:remaining])
		b.truncated = true
		return len(s), nil
	}
	_, _ = b.buf.WriteString(s)
	return len(s), nil
}

func (b *LimitedCaptureBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *LimitedCaptureBuffer) Truncated() bool {
	return b.truncated
}

// CapturingResponseWriter 包装 gin.ResponseWriter，在向客户端写出数据的同时，
// 把同一份数据写入内部 Buf，用于捕获"返回给下游的原始响应体"。
// 通过嵌入 gin.ResponseWriter 接口，Flush / WriteHeader 等流式方法会自动委托给底层 writer，
// 保证 SSE 等流式响应正常工作。仅重写 Write 与 WriteString 以同步写入 Buf。
type CapturingResponseWriter struct {
	gin.ResponseWriter
	Buf *LimitedCaptureBuffer
}

// ReadFrom deliberately uses Write instead of delegating to the embedded
// writer's io.ReaderFrom implementation. io.Copy prefers ReaderFrom when it
// is available, which would otherwise bypass diagnostic capture.
func (w *CapturingResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	var total int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			written, writeErr := w.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

// Write 同步写入底层 writer 和 Buf
func (w *CapturingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if n > 0 {
		_, _ = w.Buf.Write(b[:n])
	}
	return n, err
}

// WriteString 同步写入底层 writer 和 Buf
func (w *CapturingResponseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if n > 0 {
		_, _ = w.Buf.WriteString(s[:min(n, len(s))])
	}
	return n, err
}

// CapturingReadCloser 包装 io.ReadCloser，在读取上游响应体的同时，
// 把读到的数据写入内部 Buf，用于捕获"上游返回的原始响应体"。
type CapturingReadCloser struct {
	Reader io.Reader
	Closer io.Closer
	Buf    *LimitedCaptureBuffer
}

// Read 读取数据并同步写入 Buf
func (c *CapturingReadCloser) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	if n > 0 {
		_, _ = c.Buf.Write(p[:n])
	}
	return n, err
}

// Close 关闭底层的 ReadCloser
func (c *CapturingReadCloser) Close() error {
	return c.Closer.Close()
}
