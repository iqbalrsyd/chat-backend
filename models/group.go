package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Group struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty"`
	Name      string               `bson:"name"`
	Admins    []primitive.ObjectID `bson:"admins"`
	Members   []primitive.ObjectID `bson:"members"`
	CreatedAt primitive.DateTime   `bson:"created_at"`
}
