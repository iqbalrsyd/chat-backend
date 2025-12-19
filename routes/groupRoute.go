package routes

import (
    "chat-backend/controllers"
    "github.com/gin-gonic/gin"
)

func GroupRoutes(router *gin.Engine) {
    group := router.Group("/groups")
    {
        group.POST("/", controllers.CreateGroup)
        group.POST("/:group_id/add_member", controllers.AddMember)
        group.POST("/:group_id/remove_member", controllers.RemoveMember)
        group.DELETE("/:group_id", controllers.DeleteGroup)
    }
}
