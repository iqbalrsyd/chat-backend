package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chat-backend/config"
	"chat-backend/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"github.com/redis/go-redis/v9"
)

type PubSubManager struct {
	redis    *config.RedisConfig
	pubsub   *redis.PubSub
	ctx      context.Context
	cancel   context.CancelFunc
	handlers map[string][]MessageHandler
}

type MessageHandler func(message *models.Message) error
type UserStatusHandler func(userID primitive.ObjectID, isOnline bool) error
type ChatActivityHandler func(chatID primitive.ObjectID, activityType string, data interface{}) error

type PubSubMessage struct {
	Type    string      `json:"type"`    // message, user_status, chat_activity
	ChatID  string      `json:"chat_id"` // For message and chat_activity events
	SenderID string     `json:"sender_id"`
	Content string      `json:"content"`
	Data    interface{} `json:"data"`    // Additional data based on type
}

// Channel names for different events
const (
	ChannelMessage       = "chat_messages"
	ChannelUserStatus    = "user_status"
	ChannelChatActivity  = "chat_activity"
)

// Message types
const (
	TypeMessage          = "message"
	TypeUserOnline       = "user_online"
	TypeUserOffline      = "user_offline"
	TypeTyping           = "typing"
	TypeStopTyping       = "stop_typing"
	TypeChatUpdated      = "chat_updated"
	TypeMemberAdded      = "member_added"
	TypeMemberRemoved    = "member_removed"
)

// NewPubSubManager creates a new Pub/Sub manager instance
func NewPubSubManager(redisConfig *config.RedisConfig) *PubSubManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &PubSubManager{
		redis:    redisConfig,
		ctx:      ctx,
		cancel:   cancel,
		handlers: make(map[string][]MessageHandler),
	}
}

// PublishMessage publishes a new message to a chat channel
func (pm *PubSubManager) PublishMessage(ctx context.Context, message *models.Message) error {
	if !pm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	channel := fmt.Sprintf("%s:%s", ChannelMessage, message.ChatID.Hex())

	pubSubMsg := PubSubMessage{
		Type:      TypeMessage,
		ChatID:    message.ChatID.Hex(),
		SenderID:  message.SenderID.Hex(),
		Content:   message.Content,
		Data:      message,
	}

	data, err := json.Marshal(pubSubMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return pm.redis.Client.Publish(ctx, channel, data).Err()
}

// PublishUserStatus publishes user online/offline status
func (pm *PubSubManager) PublishUserStatus(ctx context.Context, userID primitive.ObjectID, isOnline bool) error {
	if !pm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	msgType := TypeUserOffline
	if isOnline {
		msgType = TypeUserOnline
	}

	pubSubMsg := PubSubMessage{
		Type:     msgType,
		SenderID: userID.Hex(),
		Data: map[string]interface{}{
			"user_id":   userID.Hex(),
			"is_online": isOnline,
		},
	}

	data, err := json.Marshal(pubSubMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal user status: %w", err)
	}

	return pm.redis.Client.Publish(ctx, ChannelUserStatus, data).Err()
}

// PublishTypingStatus publishes typing status for a chat
func (pm *PubSubManager) PublishTypingStatus(ctx context.Context, chatID, userID primitive.ObjectID, isTyping bool) error {
	if !pm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	channel := fmt.Sprintf("%s:%s", ChannelMessage, chatID.Hex())

	msgType := TypeStopTyping
	if isTyping {
		msgType = TypeTyping
	}

	pubSubMsg := PubSubMessage{
		Type:      msgType,
		ChatID:    chatID.Hex(),
		SenderID:  userID.Hex(),
		Data: map[string]interface{}{
			"user_id": userID.Hex(),
			"chat_id": chatID.Hex(),
		},
	}

	data, err := json.Marshal(pubSubMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal typing status: %w", err)
	}

	return pm.redis.Client.Publish(ctx, channel, data).Err()
}

// PublishChatActivity publishes chat-related activities
func (pm *PubSubManager) PublishChatActivity(ctx context.Context, chatID primitive.ObjectID, activityType string, data interface{}) error {
	if !pm.redis.IsConnected() {
		return fmt.Errorf("redis not connected")
	}

	channel := fmt.Sprintf("%s:%s", ChannelChatActivity, chatID.Hex())

	pubSubMsg := PubSubMessage{
		Type:   activityType,
		ChatID: chatID.Hex(),
		Data:   data,
	}

	messageData, err := json.Marshal(pubSubMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal chat activity: %w", err)
	}

	return pm.redis.Client.Publish(ctx, channel, messageData).Err()
}

// SubscribeToChat subscribes to messages for a specific chat
func (pm *PubSubManager) SubscribeToChat(chatID primitive.ObjectID) (*redis.PubSub, error) {
	if !pm.redis.IsConnected() {
		return nil, fmt.Errorf("redis not connected")
	}

	channel := fmt.Sprintf("%s:%s", ChannelMessage, chatID.Hex())
	pubsub := pm.redis.Client.Subscribe(pm.ctx, channel)

	// Test subscription
	_, err := pubsub.Receive(pm.ctx)
	if err != nil {
		pubsub.Close()
		return nil, fmt.Errorf("failed to subscribe to channel: %w", err)
	}

	log.Printf("Subscribed to chat channel: %s", channel)
	return pubsub, nil
}

// SubscribeToUserStatus subscribes to user status changes
func (pm *PubSubManager) SubscribeToUserStatus() (*redis.PubSub, error) {
	if !pm.redis.IsConnected() {
		return nil, fmt.Errorf("redis not connected")
	}

	pubsub := pm.redis.Client.Subscribe(pm.ctx, ChannelUserStatus)

	// Test subscription
	_, err := pubsub.Receive(pm.ctx)
	if err != nil {
		pubsub.Close()
		return nil, fmt.Errorf("failed to subscribe to user status channel: %w", err)
	}

	log.Println("Subscribed to user status channel")
	return pubsub, nil
}

// SubscribeToChatActivity subscribes to chat activity for a specific chat
func (pm *PubSubManager) SubscribeToChatActivity(chatID primitive.ObjectID) (*redis.PubSub, error) {
	if !pm.redis.IsConnected() {
		return nil, fmt.Errorf("redis not connected")
	}

	channel := fmt.Sprintf("%s:%s", ChannelChatActivity, chatID.Hex())
	pubsub := pm.redis.Client.Subscribe(pm.ctx, channel)

	// Test subscription
	_, err := pubsub.Receive(pm.ctx)
	if err != nil {
		pubsub.Close()
		return nil, fmt.Errorf("failed to subscribe to chat activity channel: %w", err)
	}

	log.Printf("Subscribed to chat activity channel: %s", channel)
	return pubsub, nil
}

// ListenToMessages starts listening to messages from a subscription
func (pm *PubSubManager) ListenToMessages(pubsub *redis.PubSub, messageHandler func(msg PubSubMessage) error) {
	ch := pubsub.Channel(pm.ctx)

	go func() {
		for {
			select {
			case msg := <-ch:
				if msg == nil {
					log.Println("PubSub channel closed")
					return
				}

				var pubSubMsg PubSubMessage
				if err := json.Unmarshal([]byte(msg.Payload), &pubSubMsg); err != nil {
					log.Printf("Failed to unmarshal pubsub message: %v", err)
					continue
				}

				if err := messageHandler(pubSubMsg); err != nil {
					log.Printf("Error handling pubsub message: %v", err)
				}

			case <-pm.ctx.Done():
				log.Println("PubSub listener context cancelled")
				return
			}
		}
	}()
}

// Unsubscribe closes the subscription
func (pm *PubSubManager) Unsubscribe(pubsub *redis.PubSub) error {
	if pubsub != nil {
		return pubsub.Close()
	}
	return nil
}

// Close stops all pubsub operations and closes connections
func (pm *PubSubManager) Close() {
	if pm.cancel != nil {
		pm.cancel()
	}
}

// GetOnlineUsersCount returns the count of online users
func (pm *PubSubManager) GetOnlineUsersCount(ctx context.Context) (int64, error) {
	if !pm.redis.IsConnected() {
		return 0, fmt.Errorf("redis not connected")
	}

	pattern := "online_users:*"
	keys, err := pm.redis.Client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}

	return int64(len(keys)), nil
}

// GetActiveChatsCount returns the count of chats with recent activity
func (pm *PubSubManager) GetActiveChatsCount(ctx context.Context) (int64, error) {
	if !pm.redis.IsConnected() {
		return 0, fmt.Errorf("redis not connected")
	}

	pattern := "messages:*"
	keys, err := pm.redis.Client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}

	return int64(len(keys)), nil
}