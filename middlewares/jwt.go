package middlewares

import (
	"chat-backend/config"
	"chat-backend/models"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var jwtSecret []byte

// Initialize JWT secret from config
func InitJWTSecret(cfg *config.Config) {
	jwtSecret = []byte(cfg.JWTSecret)
}

// GetTokenFromHeaderOrCookie retrieves JWT token from Authorization header or cookies
func GetTokenFromHeaderOrCookie(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), nil
	}

	cookie, err := c.Cookie("jwt")
	if err == nil {
		return cookie, nil
	}

	return "", errors.New("no token provided")
}

// VerifyToken verifies the JWT token
func VerifyToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return jwtSecret, nil
	})
}

// Protect is the middleware to protect routes using JWT
func Protect(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from header or cookie
		tokenString, err := GetTokenFromHeaderOrCookie(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authorized, no token provided"})
			c.Abort()
			return
		}

		// Verify the token
		token, err := VerifyToken(tokenString)
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token or token expired"})
			c.Abort()
			return
		}

		// Extract claims from the token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token"})
			c.Abort()
			return
		}

		// Get user ID from claims
		userID, ok := claims["user_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token claims"})
			c.Abort()
			return
		}

		// Fetch user from the database
		var user models.User
		err = db.Collection("users").FindOne(context.TODO(), bson.M{"_id": userID}).Decode(&user)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "User not found or unauthorized"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Error finding user"})
			}
			c.Abort()
			return
		}

		// Attach user to the context
		c.Set("user", user)

		// Call the next handler
		c.Next()
	}
}
