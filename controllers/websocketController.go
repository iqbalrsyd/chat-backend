package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"chat-backend/config"
	"chat-backend/models"
	"chat-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebSocketUpgrader configures the WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should implement proper origin checking
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client represents a WebSocket client
type Client struct {
	ID       primitive.ObjectID
	ChatID   primitive.ObjectID
	Conn     *websocket.Conn
	Send     chan []byte
	Mu       sync.Mutex
	IsClosed bool
}

// WebSocketHub manages active WebSocket connections
type WebSocketHub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
	config     *config.Config
}

// Message represents a WebSocket message
type WSMessage struct {
	Type      string      `json:"type"`       // message, typing, status, error
	ChatID    string      `json:"chat_id,omitempty"`
	SenderID  string      `json:"sender_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(config *config.Config) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		config:     config,
	}
}

// Run starts the WebSocket hub
func (hub *WebSocketHub) Run() {
	for {
		select {
		case client := <-hub.register:
			hub.mu.Lock()
			hub.clients[client] = true
			hub.mu.Unlock()
			log.Printf("Client connected: %s to chat: %s", client.ID.Hex(), client.ChatID.Hex())

			// Set user as online
			cacheManager := utils.NewCacheManager(hub.config.Redis)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cacheManager.SetUserOnline(ctx, client.ID, 5*time.Minute)
			cancel()

			// Publish user online status
			pubSubManager := utils.NewPubSubManager(hub.config.Redis)
			pubSubManager.PublishUserStatus(ctx, client.ID, true)

		case client := <-hub.unregister:
			hub.mu.Lock()
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.Send)
				log.Printf("Client disconnected: %s from chat: %s", client.ID.Hex(), client.ChatID.Hex())

				// Set user as offline
				cacheManager := utils.NewCacheManager(hub.config.Redis)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cacheManager.SetUserOffline(ctx, client.ID)
				cancel()

				// Publish user offline status
				pubSubManager := utils.NewPubSubManager(hub.config.Redis)
				pubSubManager.PublishUserStatus(ctx, client.ID, false)
			}
			hub.mu.Unlock()

		case message := <-hub.broadcast:
			hub.mu.RLock()
			for client := range hub.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(hub.clients, client)
				}
			}
			hub.mu.RUnlock()
		}
	}
}

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(hub *WebSocketHub) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Get user ID from query parameter or JWT token
		userIDParam := c.Query("user_id")
		chatIDParam := c.Query("chat_id")

		if userIDParam == "" || chatIDParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and chat_id are required"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		chatID, err := primitive.ObjectIDFromHex(chatIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chat ID"})
			return
		}

		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		// Create new client
		client := &Client{
			ID:       userID,
			ChatID:   chatID,
			Conn:     conn,
			Send:     make(chan []byte, 256),
			IsClosed: false,
		}

		// Register client with hub
		hub.register <- client

		// Start goroutines for reading and writing
		go hub.writePump(client)
		go hub.readPump(client, hub)
		go hub.listenToRedisMessages(client)
	})
}

// readPump reads messages from WebSocket connection
func (hub *WebSocketHub) readPump(client *Client, hubRef *WebSocketHub) {
	defer func() {
		hubRef.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse incoming message
		var wsMessage WSMessage
		if err := json.Unmarshal(messageBytes, &wsMessage); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		// Handle different message types
		switch wsMessage.Type {
		case "message":
			// Handle new message
			hub.handleNewMessage(client, wsMessage)
		case "typing":
			// Handle typing indicator
			hub.handleTyping(client, wsMessage)
		case "stop_typing":
			// Handle stop typing indicator
			hub.handleStopTyping(client, wsMessage)
		}
	}
}

// writePump writes messages to WebSocket connection
func (hub *WebSocketHub) writePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// listenToRedisMessages subscribes to Redis Pub/Sub for real-time messages
func (hub *WebSocketHub) listenToRedisMessages(client *Client) {
	pubSubManager := utils.NewPubSubManager(hub.config.Redis)

	// Subscribe to chat messages
	pubsub, err := pubSubManager.SubscribeToChat(client.ChatID)
	if err != nil {
		log.Printf("Failed to subscribe to chat messages: %v", err)
		return
	}
	defer pubsub.Unsubscribe(hub.config.Redis.Client.Context())

	// Listen to messages
	pubSubManager.ListenToMessages(pubsub, func(msg utils.PubSubMessage) error {
		switch msg.Type {
		case utils.TypeMessage:
			// Forward chat messages to client
			wsMessage := WSMessage{
				Type:      "message",
				ChatID:    msg.ChatID,
				SenderID:  msg.SenderID,
				Content:   msg.Content,
				Data:      msg.Data,
				Timestamp: time.Now().Unix(),
			}

			messageBytes, err := json.Marshal(wsMessage)
			if err != nil {
				return err
			}

			select {
			case client.Send <- messageBytes:
			default:
				// Client buffer is full, skip message
			}

		case utils.TypeTyping, utils.TypeStopTyping:
			// Forward typing indicators
			wsMessage := WSMessage{
				Type:      msg.Type,
				ChatID:    msg.ChatID,
				SenderID:  msg.SenderID,
				Timestamp: time.Now().Unix(),
				Data:      msg.Data,
			}

			messageBytes, err := json.Marshal(wsMessage)
			if err != nil {
				return err
			}

			select {
			case client.Send <- messageBytes:
			default:
			}
		}

		return nil
	})
}

// handleNewMessage processes new messages from clients
func (hub *WebSocketHub) handleNewMessage(client *Client, wsMessage WSMessage) {
	// Create message object
	message := models.Message{
		ChatID:    client.ChatID,
		SenderID:  client.ID,
		Content:   wsMessage.Content,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	// Save to MongoDB
	collection := hub.config.DB.Collection("messages")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := collection.InsertOne(ctx, message)
	if err != nil {
		// Send error back to client
		errorMsg := WSMessage{
			Type:      "error",
			Content:   "Failed to send message",
			Timestamp: time.Now().Unix(),
		}

		if errorBytes, marshalErr := json.Marshal(errorMsg); marshalErr == nil {
			select {
			case client.Send <- errorBytes:
			default:
			}
		}
		return
	}

	message.ID = result.InsertedID.(primitive.ObjectID)

	// Invalidate cache
	cacheManager := utils.NewCacheManager(hub.config.Redis)
	cacheManager.InvalidateMessagesCache(ctx, client.ChatID)

	// Publish to Redis Pub/Sub
	pubSubManager := utils.NewPubSubManager(hub.config.Redis)
	if err := pubSubManager.PublishMessage(ctx, &message); err != nil {
		log.Printf("Failed to publish message to Redis: %v", err)
	}

	// Send confirmation back to sender
	confirmMsg := WSMessage{
		Type:      "message_sent",
		ChatID:    client.ChatID.Hex(),
		SenderID:  client.ID.Hex(),
		Content:   wsMessage.Content,
		Timestamp: time.Now().Unix(),
		Data:      message,
	}

	if confirmBytes, err := json.Marshal(confirmMsg); err == nil {
		select {
		case client.Send <- confirmBytes:
		default:
		}
	}
}

// handleTyping processes typing indicators
func (hub *WebSocketHub) handleTyping(client *Client, wsMessage WSMessage) {
	pubSubManager := utils.NewPubSubManager(hub.config.Redis)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pubSubManager.PublishTypingStatus(ctx, client.ChatID, client.ID, true); err != nil {
		log.Printf("Failed to publish typing status: %v", err)
	}
}

// handleStopTyping processes stop typing indicators
func (hub *WebSocketHub) handleStopTyping(client *Client, wsMessage WSMessage) {
	pubSubManager := utils.NewPubSubManager(hub.config.Redis)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pubSubManager.PublishTypingStatus(ctx, client.ChatID, client.ID, false); err != nil {
		log.Printf("Failed to publish stop typing status: %v", err)
	}
}

// GetConnectedUsers returns the list of connected users for monitoring
func (hub *WebSocketHub) GetConnectedUsers() []primitive.ObjectID {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	var users []primitive.ObjectID
	for client := range hub.clients {
		users = append(users, client.ID)
	}

	return users
}

// GetActiveChats returns the list of chats with active connections
func (hub *WebSocketHub) GetActiveChats() []primitive.ObjectID {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	chatMap := make(map[primitive.ObjectID]bool)
	for client := range hub.clients {
		chatMap[client.ChatID] = true
	}

	var chats []primitive.ObjectID
	for chatID := range chatMap {
		chats = append(chats, chatID)
	}

	return chats
}