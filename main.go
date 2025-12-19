package main

import (
	"chat-backend/config"
	"chat-backend/controllers"
	"chat-backend/routes"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load konfigurasi
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("❌ Failed to load configuration:", err)
	}

	// Koneksi ke database
	err = cfg.ConnectDB()
	if err != nil {
		log.Fatal("❌ Failed to connect to the database:", err)
	}

	// Koneksi ke Redis
	err = cfg.ConnectRedis()
	if err != nil {
		log.Fatal("❌ Failed to connect to Redis:", err)
	}

	// Create WebSocket hub
	wsHub := controllers.NewWebSocketHub(cfg)
	go wsHub.Run()

	// Inisialisasi router
	router := gin.Default()

	// Middleware
	// protected := router.Group("/api")
	// protected.Use(middlewares.JWTMiddleware()) // Middleware cukup dipanggil tanpa parameter manual

	// Register routes
	routes.GroupRoutes(router)
	routes.MessageRoutes(router, cfg)
	routes.NotificationRoutes(router)
	routes.UserRoutes(router)
	routes.ChatRoutes(router, cfg)
	routes.SetupWebSocketRoutes(router, cfg, wsHub)

	// Add middleware to pass config to controllers
	router.Use(func(c *gin.Context) {
		c.Set("config", cfg)
		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		redisConnected := cfg.Redis.IsConnected()
		c.JSON(200, gin.H{
			"status": "healthy",
			"mongodb": "connected",
			"redis": map[string]interface{}{
				"connected": redisConnected,
				"active_connections": len(wsHub.GetConnectedUsers()),
				"active_chats": len(wsHub.GetActiveChats()),
			},
		})
	})

	// Graceful shutdown
	go func() {
		log.Println("🚀 Server running on port 8080")
		if err := router.Run(":8080"); err != nil {
			log.Fatal("❌ Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🔄 Shutting down server...")

	// Cleanup
	wsHub.Close()
	cfg.Disconnect()

	log.Println("✅ Server shutdown complete")
}
