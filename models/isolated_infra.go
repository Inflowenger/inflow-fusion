package models

import "github.com/nats-io/jwt/v2"

type Account struct {
	ID        string         `json:"id" bson:"_id,omitempty"`
	Name      string         `json:"name" bson:"name"`
	Seed      string         `json:"seed" bson:"seed"`
	Pub       string         `json:"pub" bson:"pub"`
	Policy    *Policy        `json:"policy" bson:"policy"`
	Spec      map[string]any `json:"spec" bson:"spec"`
	CreatedAt int64          `json:"createdAt" bson:"createdAt"`
	Status    int8           `json:"status" bson:"status"`
}

type Policy struct {
	Imports   []jwt.Import `json:"imports" bson:"imports"`
	Exports   []jwt.Export `json:"exports" bson:"exports"`
	ConnLimit int64        `json:"connection_limit" bson:"connection_limit"`
}