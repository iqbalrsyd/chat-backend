package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"net/http"
	"time"

	"chat-backend/config"
	"chat-backend/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// RegisterUser handles user registration
func RegisterUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	user.Password = hashPassword(user.Password)

	token := generateVerificationToken()

	hashedToken := sha256.Sum256([]byte(token))
	user.VerificationToken = hex.EncodeToString(hashedToken[:])
	user.VerificationExpires = time.Now().Add(time.Minute * 30) // Expiry in 30 minutes
	user.IsVerified = false

	collection := config.DB.Database("chatDB").Collection("users")
	_, err := collection.InsertOne(c, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	err = sendVerificationEmail(user.Email, token, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Registration successful. Please verify your email."})
}

// VerifyEmail handles email verification
func VerifyEmail(c *gin.Context) {
	token := c.Param("token")

	// Hash the token to match with database
	hashedToken := sha256.Sum256([]byte(token))
	hexToken := hex.EncodeToString(hashedToken[:])

	// Find the user with this token and check expiration
	collection := config.DB.Database("chatDB").Collection("users")
	filter := bson.M{"verificationToken": hexToken, "verificationExpiry": bson.M{"$gt": time.Now()}}
	var user models.User
	err := collection.FindOne(c, filter).Decode(&user)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Update user to be verified
	update := bson.M{
		"$set":   bson.M{"isVerified": true},
		"$unset": bson.M{"verificationToken": "", "verificationExpiry": ""},
	}
	_, err = collection.UpdateOne(c, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

// Helper functions

func hashPassword(password string) string {
	// Dummy password hashing for demo purposes
	h := sha256.New()
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func generateVerificationToken() string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	token := make([]rune, 32)
	for i := range token {
		token[i] = letters[rand.Intn(len(letters))]
	}
	return string(token)
}

func sendVerificationEmail(email, token string, req *http.Request) error {
	subject := "Verify your email"
	message := "Click on the following link to verify your email: " +
		req.Host + "/user/verify/" + token

	return config.SendEmail(email, subject, message)
}

// LoginUser handles user login and JWT generation
func LoginUser(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	var loginRequest struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	collection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user models.User
	err := collection.FindOne(ctx, bson.M{"username": loginRequest.Username}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginRequest.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := middleware.GenerateJWT(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// BlockUser handles blocking another user
func BlockUser(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	var blockRequest struct {
		UserID      string `json:"user_id" binding:"required"`
		BlockUserID string `json:"block_user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&blockRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	userID, err := primitive.ObjectIDFromHex(blockRequest.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	blockUserID, err := primitive.ObjectIDFromHex(blockRequest.BlockUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid blocked user ID"})
		return
	}

	collection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{"$addToSet": bson.M{"blocked_users": blockUserID}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User blocked successfully"})
}

// SearchUsers handles searching for users by username or email
func SearchUsers(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)
	query := c.Query("query")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	collection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"$or": []bson.M{
			{"username": bson.M{"$regex": query, "$options": "i"}},
			{"email_id": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetActiveList retrieves users currently online
func GetActiveList(c *gin.Context) {
	db := c.MustGet("db").(*mongo.Database)

	collection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"status": "online"}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve online users"})
		return
	}

	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func UpdateUserSettings(c *gin.Context) {
	userID := c.Param("userID")
	var settings models.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = services.UpdateSettings(objectID, settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
