package middlewares

import (
	"net/http"
	"strings"

	"chat-backend/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// WebSocketAuthMiddleware validates JWT token for WebSocket connections
func WebSocketAuthMiddleware(appConfig *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from query parameter (for WebSocket connections)
		token := c.Query("token")

		if token == "" {
			// Also try to get from Authorization header
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
			c.Abort()
			return
		}

		// Parse and validate JWT token
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(appConfig.JWTSecret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Extract user ID from token claims
		userID, ok := claims["user_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Set user ID in context for use in handlers
		c.Set("user_id", userID)
		c.Next()
	}
}

// CORSWebSocketMiddleware handles CORS for WebSocket connections
func CORSWebSocketMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// In production, implement proper origin validation
		// For now, allow all origins
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// WebSocketLimiterMiddleware implements basic rate limiting for WebSocket connections
func WebSocketLimiterMiddleware() gin.HandlerFunc {
	// This is a simple implementation - in production, you'd want to use
	// a more sophisticated rate limiting solution with Redis
	return func(c *gin.Context) {
		// For now, just pass through - rate limiting can be implemented later
		c.Next()
	}
}