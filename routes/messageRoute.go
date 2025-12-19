package routes

import (
    "chat-backend/config"
    "chat-backend/controllers"
    "github.com/gin-gonic/gin"
)

func MessageRoutes(router *gin.Engine, appConfig *config.Config) {
    message := router.Group("/messages")
    {
        message.POST("/", func(c *gin.Context) {
            controllers.SendMessage(c, appConfig)
        })
        message.GET("/:chat_id", func(c *gin.Context) {
            controllers.GetMessages(c, appConfig)
        })
    }
}
