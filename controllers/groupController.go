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
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateGroup creates a new group chat
func CreateGroup(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	var groupData struct {
		Name      string               `json:"name"`
		CreatorID primitive.ObjectID   `json:"creator_id"`
		Members   []primitive.ObjectID `json:"members"`
	}

	if err := c.ShouldBindJSON(&groupData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := models.Group{
		Name:      groupData.Name,
		Admins:    []primitive.ObjectID{groupData.CreatorID},
		Members:   append(groupData.Members, groupData.CreatorID),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	collection := db.Collection("groups")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	group.ID = result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusOK, group)
}

// AddMember adds a member to the group chat
func AddMember(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	groupID, _ := primitive.ObjectIDFromHex(c.Param("group_id"))
	var userData struct {
		UserID primitive.ObjectID `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&userData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := db.Collection("groups")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.UpdateOne(ctx, bson.M{"_id": groupID}, bson.M{
		"$addToSet": bson.M{"members": userData.UserID},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}

// RemoveMember removes a member from the group chat
func RemoveMember(c *gin.Context) {
	groupID, _ := primitive.ObjectIDFromHex(c.Param("group_id"))
	var userData struct {
		UserID primitive.ObjectID `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&userData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("groups")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.UpdateOne(ctx, bson.M{"_id": groupID}, bson.M{
		"$pull": bson.M{"members": userData.UserID},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// DeleteGroup deletes a group
func DeleteGroup(c *gin.Context) {
	groupID, _ := primitive.ObjectIDFromHex(c.Param("group_id"))

	collection := config.DB.Collection("groups")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.DeleteOne(ctx, bson.M{"_id": groupID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}
