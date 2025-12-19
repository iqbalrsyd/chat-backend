package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"chat-backend/models"

	"github.com/gin-gonic/gin"
)

func VerifyEmail(c *gin.Context) {
	token := c.Param("token")
	hashedToken := hashToken(token)

	// Replace with actual user retrieval logic from database
	user := getUserByVerificationToken(hashedToken)
	if user == nil || user.VerificationExpires.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Token is invalid or expired"})
		return
	}

	user.IsVerified = true
	user.VerificationToken = ""
	user.VerificationExpires = time.Time{}

	saveUser(user)

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Placeholder functions, replace with actual implementations
func getUserByVerificationToken(token string) *models.User {
	// Retrieve user based on hashed token from database
	return nil // Replace with actual user object
}

func saveUser(user *models.User) {
	// Save the updated user object back to the database
}
