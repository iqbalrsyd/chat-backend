package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty"`
	Username            string               `bson:"username"`
	Password            string               `bson:"password"`
	PasswordHash        string               `bson:"password_hash"`
	Firstname           string               `bson:"firstname"`
	Lastname            string               `bson:"lastname"`
	Birthdate           time.Time            `bson:"birthdate"`
	ProfileName         string               `bson:"profile_name"`
	Email               string               `bson:"email_id"`
	Status              string               `bson:"status"`
	PhoneNumber         string               `bson:"phone_number"`
	ProfilePic          string               `bson:"image"`
	BlockedUsers        []primitive.ObjectID `bson:"blocked_users"`
	IsVerified          bool                 `bson:"isVerified"`
	VerificationToken   string               `bson:"verificationToken,omitempty"`
	VerificationExpires time.Time            `bson:"verificationExpiry,omitempty"`
}
