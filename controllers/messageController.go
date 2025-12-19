package controllers

import (
    "context"
    "net/http"
    "time"

    "chat-backend/config"
    "chat-backend/models"
    "chat-backend/utils"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "github.com/gin-gonic/gin"
)

// SendMessage sends a message to a chat (individual or group) with caching and real-time delivery
func SendMessage(c *gin.Context, appConfig *config.Config) {
    var messageData struct {
        ChatID   primitive.ObjectID `json:"chat_id"`
        SenderID primitive.ObjectID `json:"sender_id"`
        Content  string             `json:"content"`
    }

    if err := c.ShouldBindJSON(&messageData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    message := models.Message{
        ChatID:    messageData.ChatID,
        SenderID:  messageData.SenderID,
        Content:   messageData.Content,
        CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
    }

    // Save to MongoDB
    collection := appConfig.DB.Collection("messages")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    result, err := collection.InsertOne(ctx, message)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
        return
    }

    message.ID = result.InsertedID.(primitive.ObjectID)

    // Initialize cache manager
    cacheManager := utils.NewCacheManager(appConfig.Redis)

    // Invalidate messages cache for this chat
    if err := cacheManager.InvalidateMessagesCache(ctx, messageData.ChatID); err != nil {
        // Log error but don't fail the request
        // In production, you might want to log this properly
    }

    // Publish message to Redis Pub/Sub for real-time delivery
    pubSubManager := utils.NewPubSubManager(appConfig.Redis)
    if err := pubSubManager.PublishMessage(ctx, &message); err != nil {
        // Log error but don't fail the request
        // The message is saved, so real-time delivery can be retried later
    }

    c.JSON(http.StatusOK, message)
}

// GetMessages retrieves all messages from a chat with caching
func GetMessages(c *gin.Context, appConfig *config.Config) {
    chatID, err := primitive.ObjectIDFromHex(c.Param("chat_id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chat ID"})
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Initialize cache manager
    cacheManager := utils.NewCacheManager(appConfig.Redis)

    // Try to get messages from cache first
    messages, err := cacheManager.GetCachedMessages(ctx, chatID)
    if err == nil && messages != nil {
        // Cache hit - return cached messages
        c.JSON(http.StatusOK, gin.H{
            "messages": messages,
            "source":   "cache",
        })
        return
    }

    // Cache miss - fetch from MongoDB
    collection := appConfig.DB.Collection("messages")

    var dbMessages []models.Message
    cursor, err := collection.Find(ctx, bson.M{"chat_id": chatID})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
        return
    }
    defer cursor.Close(ctx)

    for cursor.Next(ctx) {
        var message models.Message
        if err := cursor.Decode(&message); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding message"})
            return
        }
        dbMessages = append(dbMessages, message)
    }

    if err := cursor.Err(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Cursor error"})
        return
    }

    // Cache the messages for 15 minutes (adjust TTL as needed)
    if err := cacheManager.CacheMessages(ctx, chatID, dbMessages, 15*time.Minute); err != nil {
        // Log error but don't fail the request
        // The messages are fetched successfully, just cache failed
    }

    c.JSON(http.StatusOK, gin.H{
        "messages": dbMessages,
        "source":   "database",
    })
}
