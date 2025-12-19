package controllers

import (
	"context"
	"net/http"
	"time"

	"chat-backend/config"
	"chat-backend/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetNotifications retrieves notifications for a specific user
func GetNotifications(c *gin.Context) {
	userID, _ := primitive.ObjectIDFromHex(c.Param("user_id"))

	collection := config.DB.Collection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}
	defer cursor.Close(ctx)

	var notifications []models.Notification
	if err := cursor.All(ctx, &notifications); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

func CreateNotification(c *gin.Context) {
	var notifData struct {
		UserID  primitive.ObjectID `json:"user_id"`
		Message string             `json:"message"`
	}

	if err := c.ShouldBindJSON(&notifData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notification := models.Notification{
		UserID:    notifData.UserID,
		Message:   notifData.Message,
		CreatedAt: time.Now(),
	}

	collection := config.DB.Collection("notifications")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification created successfully"})
}
