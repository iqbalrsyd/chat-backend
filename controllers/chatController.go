package controllers

import (
	"context"
	"net/http"
	"time"

	"chat-backend/config"
	"chat-backend/models"
	"chat-backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateChat creates a new chat (individual or group)
func CreateChat(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	var request struct {
		Type    string   `json:"type" binding:"required"`    // Chat type: "individual" or "group"
		Members []string `json:"members" binding:"required"` // List of member IDs
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Convert member IDs from string to ObjectID
	var memberIDs []primitive.ObjectID
	for _, id := range request.Members {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID format"})
			return
		}
		memberIDs = append(memberIDs, objectID)
	}

	// Prepare chat document
	chat := models.Chat{
		Type:      request.Type,
		Members:   memberIDs,
		CreatedAt: time.Now(),
	}

	// Insert the chat into the database
	collection := db.Collection("chats")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, chat)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat"})
		return
	}

	chat.ID = result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusOK, gin.H{"message": "Chat created successfully", "chat": chat})
}

// GetChatByID retrieves a chat by its ID with caching
func GetChatByID(c *gin.Context, appConfig *config.Config) {
	chatIDParam := c.Param("chatID")

	// Convert the chat ID from string to ObjectID
	chatID, err := primitive.ObjectIDFromHex(chatIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chat ID"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize cache manager
	cacheManager := utils.NewCacheManager(appConfig.Redis)

	// Try to get chat from cache first
	var chat models.Chat
	if err := cacheManager.GetCachedChat(ctx, chatID, &chat); err == nil {
		// Cache hit - return cached chat
		c.JSON(http.StatusOK, gin.H{
			"chat":   chat,
			"source": "cache",
		})
		return
	}

	// Cache miss - fetch from MongoDB
	collection := appConfig.DB.Collection("chats")
	err = collection.FindOne(ctx, bson.M{"_id": chatID}).Decode(&chat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat"})
		return
	}

	// Cache the chat for 30 minutes (adjust TTL as needed)
	if err := cacheManager.CacheChat(ctx, chat, chatID, 30*time.Minute); err != nil {
		// Log error but don't fail the request
		// The chat is fetched successfully, just cache failed
	}

	c.JSON(http.StatusOK, gin.H{
		"chat":   chat,
		"source": "database",
	})
}

// GetUserChats retrieves all chats for a user with caching
func GetUserChats(c *gin.Context, appConfig *config.Config) {
	userIDParam := c.Param("userID")

	// Convert the user ID from string to ObjectID
	userID, err := primitive.ObjectIDFromHex(userIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize cache manager
	cacheManager := utils.NewCacheManager(appConfig.Redis)

	// Try to get user chats from cache first
	var chats []models.Chat
	if err := cacheManager.GetCachedUserChats(ctx, userID, &chats); err == nil && len(chats) > 0 {
		// Cache hit - return cached chats
		c.JSON(http.StatusOK, gin.H{
			"chats":  chats,
			"source": "cache",
		})
		return
	}

	// Cache miss - fetch from MongoDB
	collection := appConfig.DB.Collection("chats")

	cursor, err := collection.Find(ctx, bson.M{
		"members": bson.M{"$in": []primitive.ObjectID{userID}},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user chats"})
		return
	}
	defer cursor.Close(ctx)

	var dbChats []models.Chat
	for cursor.Next(ctx) {
		var chat models.Chat
		if err := cursor.Decode(&chat); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding chat"})
			return
		}
		dbChats = append(dbChats, chat)
	}

	if err := cursor.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cursor error"})
		return
	}

	// Cache the user chats for 20 minutes (adjust TTL as needed)
	if err := cacheManager.CacheUserChats(ctx, userID, convertChatsToInterface(dbChats), 20*time.Minute); err != nil {
		// Log error but don't fail the request
		// The chats are fetched successfully, just cache failed
	}

	c.JSON(http.StatusOK, gin.H{
		"chats":  dbChats,
		"source": "database",
	})
}

// Helper function to convert chat slice to interface slice for caching
func convertChatsToInterface(chats []models.Chat) []interface{} {
	result := make([]interface{}, len(chats))
	for i, chat := range chats {
		result[i] = chat
	}
	return result
}

// InvalidateChatCache invalidates cache for a specific chat (helper function)
func InvalidateChatCache(ctx context.Context, appConfig *config.Config, chatID primitive.ObjectID) error {
	cacheManager := utils.NewCacheManager(appConfig.Redis)

	// Invalidate chat cache
	if err := cacheManager.InvalidateChatCache(ctx, chatID); err != nil {
		return err
	}

	// Invalidate messages cache for this chat
	if err := cacheManager.InvalidateMessagesCache(ctx, chatID); err != nil {
		return err
	}

	// Get chat details to invalidate member chat caches
	var chat models.Chat
	collection := appConfig.DB.Collection("chats")
	if err := collection.FindOne(ctx, bson.M{"_id": chatID}).Decode(&chat); err == nil {
		// Invalidate chat list cache for all members
		for _, memberID := range chat.Members {
			cacheManager.InvalidateUserChatsCache(ctx, memberID)
		}
	}

	return nil
}
