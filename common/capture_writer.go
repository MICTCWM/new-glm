package common

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

// CapturingResponseWriter 包装 gin.ResponseWriter，在向客户端写出数据的同时，
// 把同一份数据写入内部 Buf，用于捕获"返回给下游的原始响应体"。
// 通过嵌入 gin.ResponseWriter 接口，Flush / WriteHeader 等流式方法会自动委托给底层 writer，
// 保证 SSE 等流式响应正常工作。仅重写 Write 与 WriteString 以同步写入 Buf。
type CapturingResponseWriter struct {
	gin.ResponseWriter
	Buf *bytes.Buffer
}

// Write 同步写入底层 writer 和 Buf
func (w *CapturingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if n > 0 {
		w.Buf.Write(b[:n])
	}
	return n, err
}

// WriteString 同步写入底层 writer 和 Buf
func (w *CapturingResponseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if n > 0 {
		w.Buf.WriteString(s[:n])
	}
	return n, err
}

// CapturingReadCloser 包装 io.ReadCloser，在读取上游响应体的同时，
// 把读到的数据写入内部 Buf，用于捕获"上游返回的原始响应体"。
type CapturingReadCloser struct {
	Reader io.Reader
	Closer io.Closer
	Buf    *bytes.Buffer
}

// Read 读取数据并同步写入 Buf
func (c *CapturingReadCloser) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	if n > 0 {
		c.Buf.Write(p[:n])
	}
	return n, err
}

// Close 关闭底层的 ReadCloser
func (c *CapturingReadCloser) Close() error {
	return c.Closer.Close()
}
