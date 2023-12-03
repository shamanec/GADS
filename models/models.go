package models

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"-" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id" bson:"_id"`
}
