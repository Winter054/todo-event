package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	EventIssued   = "captcha.issued"
	EventVerified = "captcha.verified"
	EventFailed   = "captcha.failed"
)

type Challenge struct {
	ID        bson.ObjectID `bson:"_id"        json:"id"`
	Question  string        `bson:"question"   json:"question"`
	Answer    int           `bson:"answer"     json:"-"`
	Verified  bool          `bson:"verified"   json:"verified"`
	IssuedAt  time.Time     `bson:"issued_at"  json:"issued_at"`
	ExpiresAt time.Time     `bson:"expires_at" json:"expires_at"`
}

func (c Challenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

type IssuedPayload struct {
	ID        bson.ObjectID `bson:"_id"        json:"id"`
	Question  string        `bson:"question"   json:"question"`
	Answer    int           `bson:"answer"     json:"answer"`
	ExpiresAt time.Time     `bson:"expires_at" json:"expires_at"`
}

type VerifiedPayload struct {
	ID bson.ObjectID `bson:"_id" json:"id"`
}

type VerifiedLokiPayload struct {
	ID        bson.ObjectID `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
}

type FailedPayload struct {
	ID     bson.ObjectID `bson:"_id"    json:"id"`
	Reason string        `bson:"reason" json:"reason"`
}
