package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetDashboardRouter(router *gin.Engine) {
	apiRouter := router.Group("/")
	apiRouter.Use(middleware.RouteTag("old_api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	apiRouter.Use(middleware.CORS())
	apiRouter.Use(middleware.TokenAuth())
	{
		apiRouter.GET("/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/v1/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/dashboard/billing/usage", controller.GetUsage)
		apiRouter.GET("/v1/dashboard/billing/usage", controller.GetUsage)
		apiRouter.GET("/v1/usage/quota", controller.GetV1UsageQuota)
	}

	// Admin-only routes
	adminRouter := router.Group("/")
	adminRouter.Use(middleware.RouteTag("dashboard_admin"))
	adminRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	adminRouter.Use(middleware.GlobalAPIRateLimit())
	adminRouter.Use(middleware.CORS())
	adminRouter.Use(middleware.TokenAuth())
	adminRouter.Use(middleware.AdminAuth())
	{
		adminRouter.GET("/dashboard/queue-status", controller.GetQueueStatus)
	}
}
