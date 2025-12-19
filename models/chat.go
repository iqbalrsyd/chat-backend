package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Chat represents a chat structure in the database
type Chat struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty"`
	Type      string               `bson:"type"` // "individual" or "group"
	Members   []primitive.ObjectID `bson:"members"`
	CreatedAt time.Time            `bson:"created_at"`
}
