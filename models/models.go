package models

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password,omitempty" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id,omitempty" bson:"_id,omitempty"`
}
