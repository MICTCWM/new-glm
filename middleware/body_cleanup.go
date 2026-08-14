package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// BodyStorageCleanup 请求体存储清理中间件
// 在请求处理完成后自动清理磁盘/内存缓存
func BodyStorageCleanup() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			// Cleanup must also run when a downstream handler panics and recovery
			// unwinds the middleware stack.
			if c.Request.MultipartForm != nil {
				if err := c.Request.MultipartForm.RemoveAll(); err != nil {
					common.SysError("failed to remove request multipart temporary files: " + err.Error())
				}
			}
			common.CleanupBodyStorage(c)

			// 清理文件缓存（URL 下载的文件等）
			service.CleanupFileSources(c)
		}()

		c.Next()
	}
}
