package models

type User struct {
	Username string `json:"username" bson:"username"`
	Email    string `json:"email" bson:"email"`
	Password string `json:"password" bson:"password"`
	Role     string `json:"role,omitempty" bson:"role"`
	ID       string `json:"_id" bson:"_id,omitempty"`
}
