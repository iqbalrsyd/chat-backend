package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	MongoURI          string
	DBName            string
	JWTSecret         string
	GoogleClientID    string
	GoogleSecret      string
	GoogleCallbackURL string
	GoogleOAuthConfig *oauth2.Config
	DB                *mongo.Database
	Redis             *RedisConfig
}

// LoadConfig membaca konfigurasi dari .env atau environment variable
func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // Tidak perlu error handling, cukup gunakan env default

	config := &Config{
		MongoURI:          getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:            getEnv("DB_NAME", "chatapp"),
		JWTSecret:         getEnv("JWT_SECRET", "your-jwt-secret"),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleSecret:      getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleCallbackURL: getEnv("GOOGLE_CALLBACK_URL", "/user/auth/google/callback"),
		Redis: &RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
	}

	// Inisialisasi Google OAuth Config
	config.GoogleOAuthConfig = &oauth2.Config{
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleSecret,
		RedirectURL:  config.GoogleCallbackURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return config, nil
}

// ConnectDB menghubungkan ke MongoDB dan mengembalikan instance database
func (c *Config) ConnectDB() error {
	clientOptions := options.Client().ApplyURI(c.MongoURI)
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return err
	}

	c.DB = client.Database(c.DBName)
	log.Println("✅ Connected to MongoDB")
	return nil
}

// ConnectRedis connects to Redis
func (c *Config) ConnectRedis() error {
	return c.Redis.ConnectRedis()
}

// Disconnect menutup koneksi MongoDB dan Redis
func (c *Config) Disconnect() error {
	var lastErr error

	// Disconnect from MongoDB
	if c.DB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.DB.Client().Disconnect(ctx); err != nil {
			lastErr = err
		}
	}

	// Disconnect from Redis
	if c.Redis != nil {
		if err := c.Redis.Disconnect(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// getEnv membaca variabel lingkungan atau menggunakan default jika tidak tersedia
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
