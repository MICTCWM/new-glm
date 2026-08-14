package middleware

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	origins := strings.Split(os.Getenv("CORS_ALLOW_ORIGINS"), ",")
	config.AllowOrigins = nil
	for _, origin := range origins {
		if origin = strings.TrimSpace(origin); origin != "" {
			if origin == "*" {
				continue
			}
			config.AllowOrigins = append(config.AllowOrigins, origin)
		}
	}
	if len(config.AllowOrigins) == 0 {
		// Do not combine credentials with a wildcard. With no explicit allowlist,
		// deny cross-origin requests and keep same-origin requests unaffected.
		config.AllowOriginFunc = func(string) bool { return false }
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
