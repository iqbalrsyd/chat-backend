package routes

import (
	"chat-backend/config"
	"chat-backend/controllers"
	"chat-backend/middlewares"

	"github.com/gin-gonic/gin"
)

// SetupWebSocketRoutes configures WebSocket routes
func SetupWebSocketRoutes(router *gin.Engine, appConfig *config.Config, hub *controllers.WebSocketHub) {
	// WebSocket group with middleware
	wsGroup := router.Group("/ws")
	wsGroup.Use(middlewares.CORSWebSocketMiddleware())
	wsGroup.Use(middlewares.WebSocketLimiterMiddleware())

	{
		// WebSocket endpoint for real-time chat
		wsGroup.GET("/chat", middlewares.WebSocketAuthMiddleware(appConfig), controllers.HandleWebSocket(hub))

		// WebSocket health check
		wsGroup.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "healthy",
				"connected_users": len(hub.GetConnectedUsers()),
				"active_chats": len(hub.GetActiveChats()),
			})
		})
	}
}