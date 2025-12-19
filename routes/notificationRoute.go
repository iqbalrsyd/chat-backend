package routes

import (
    "chat-backend/controllers"
    "github.com/gin-gonic/gin"
)

func NotificationRoutes(router *gin.Engine) {
    notifications := router.Group("/notifications")
    {
        notifications.GET("/:user_id", controllers.GetNotifications)
        notifications.POST("/", controllers.CreateNotification)
    }
}
