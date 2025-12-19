package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	ChatID    primitive.ObjectID `bson:"chat_id"`
	SenderID  primitive.ObjectID `bson:"sender_id"`
	Content   string             `bson:"content"`
	CreatedAt primitive.DateTime `bson:"created_at"`
}
