package models

import "time"

type CustomAction struct {
	ID          string    `json:"id" bson:"_id,omitempty"`
	Name        string    `json:"name" bson:"name"`
	Description string    `json:"description" bson:"description"`
	ActionType  string    `json:"action_type" bson:"action_type"` // pinch_in, pinch_out, double_tap
	IsFavorite  bool      `json:"is_favorite" bson:"is_favorite"`
	CreatedBy   string    `json:"created_by" bson:"created_by"`
	Tenant      string    `json:"tenant" bson:"tenant"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}

type ExecuteCustomActionRequest struct {
	ActionType string  `json:"action_type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}
