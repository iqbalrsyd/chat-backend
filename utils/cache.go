package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chat-backend/config"
	"chat-backend/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CacheManager struct {
	redis *config.RedisConfig
}

// NewCacheManager creates a new cache manager instance
func NewCacheManager(redisConfig *config.RedisConfig) *CacheManager {
	return &CacheManager{
		redis: redisConfig,
	}
}

// CacheKey constants
const (
	CacheKeyMessages    = "messages"
	CacheKeyChat        = "chat"
	CacheKeyUserChats   = "user_chats"
	CacheKeyOnlineUsers = "online_users"
	CacheKeyUserSession = "user_session"
)

// CacheMessages caches messages for a specific chat
func (cm *CacheManager) CacheMessages(ctx context.Context, chatID primitive.ObjectID, messages []models.Message, ttl time.Duration) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyMessages, chatID.Hex())

	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	return cm.redis.Client.Set(ctx, key, data, ttl).Err()
}

// GetCachedMessages retrieves cached messages for a specific chat
func (cm *CacheManager) GetCachedMessages(ctx context.Context, chatID primitive.ObjectID) ([]models.Message, error) {
	if !cm.redis.IsConnected() {
		return nil, fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyMessages, chatID.Hex())

	data, err := cm.redis.Client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var messages []models.Message
	if err := json.Unmarshal([]byte(data), &messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return messages, nil
}

// InvalidateMessagesCache invalidates the messages cache for a specific chat
func (cm *CacheManager) InvalidateMessagesCache(ctx context.Context, chatID primitive.ObjectID) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyMessages, chatID.Hex())
	return cm.redis.Client.Del(ctx, key).Err()
}

// CacheChat caches a chat object
func (cm *CacheManager) CacheChat(ctx context.Context, chat interface{}, chatID primitive.ObjectID, ttl time.Duration) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyChat, chatID.Hex())

	data, err := json.Marshal(chat)
	if err != nil {
		return fmt.Errorf("failed to marshal chat: %w", err)
	}

	return cm.redis.Client.Set(ctx, key, data, ttl).Err()
}

// GetCachedChat retrieves a cached chat object
func (cm *CacheManager) GetCachedChat(ctx context.Context, chatID primitive.ObjectID, target interface{}) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyChat, chatID.Hex())

	data, err := cm.redis.Client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), target)
}

// InvalidateChatCache invalidates the chat cache for a specific chat
func (cm *CacheManager) InvalidateChatCache(ctx context.Context, chatID primitive.ObjectID) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyChat, chatID.Hex())
	return cm.redis.Client.Del(ctx, key).Err()
}

// CacheUserChats caches a user's chat list
func (cm *CacheManager) CacheUserChats(ctx context.Context, userID primitive.ObjectID, chats []interface{}, ttl time.Duration) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserChats, userID.Hex())

	data, err := json.Marshal(chats)
	if err != nil {
		return fmt.Errorf("failed to marshal user chats: %w", err)
	}

	return cm.redis.Client.Set(ctx, key, data, ttl).Err()
}

// GetCachedUserChats retrieves a user's cached chat list
func (cm *CacheManager) GetCachedUserChats(ctx context.Context, userID primitive.ObjectID, target interface{}) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserChats, userID.Hex())

	data, err := cm.redis.Client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), target)
}

// InvalidateUserChatsCache invalidates a user's chat list cache
func (cm *CacheManager) InvalidateUserChatsCache(ctx context.Context, userID primitive.ObjectID) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserChats, userID.Hex())
	return cm.redis.Client.Del(ctx, key).Err()
}

// SetUserOnline marks a user as online with TTL
func (cm *CacheManager) SetUserOnline(ctx context.Context, userID primitive.ObjectID, ttl time.Duration) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyOnlineUsers, userID.Hex())
	return cm.redis.Client.Set(ctx, key, "online", ttl).Err()
}

// SetUserOffline marks a user as offline
func (cm *CacheManager) SetUserOffline(ctx context.Context, userID primitive.ObjectID) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyOnlineUsers, userID.Hex())
	return cm.redis.Client.Del(ctx, key).Err()
}

// IsUserOnline checks if a user is online
func (cm *CacheManager) IsUserOnline(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	if !cm.redis.IsConnected() {
		return false, fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyOnlineUsers, userID.Hex())
	_, err := cm.redis.Client.Get(ctx, key).Result()
	if err != nil {
		return false, nil // Not found means offline
	}

	return true, nil
}

// SetUserSession caches user session data
func (cm *CacheManager) SetUserSession(ctx context.Context, userID primitive.ObjectID, sessionData interface{}, ttl time.Duration) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserSession, userID.Hex())

	data, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return cm.redis.Client.Set(ctx, key, data, ttl).Err()
}

// GetUserSession retrieves cached user session data
func (cm *CacheManager) GetUserSession(ctx context.Context, userID primitive.ObjectID, target interface{}) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserSession, userID.Hex())

	data, err := cm.redis.Client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), target)
}

// InvalidateUserSession invalidates a user's session cache
func (cm *CacheManager) InvalidateUserSession(ctx context.Context, userID primitive.ObjectID) error {
	if !cm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	key := cm.redis.GetCacheKey(CacheKeyUserSession, userID.Hex())
	return cm.redis.Client.Del(ctx, key).Err()
}