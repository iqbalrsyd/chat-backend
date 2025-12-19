package routes

import (
	"chat-backend/config"
	"chat-backend/controllers"

	"github.com/gin-gonic/gin"
)

func ChatRoutes(router *gin.Engine, appConfig *config.Config) {
	chatGroup := router.Group("/chats")
	{
		chatGroup.POST("/", controllers.CreateChat)
		chatGroup.GET("/:chatID", func(c *gin.Context) {
			controllers.GetChatByID(c, appConfig)
		})
		chatGroup.GET("/user/:userID", func(c *gin.Context) {
			controllers.GetUserChats(c, appConfig)
		})
	}
}
