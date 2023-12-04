package router

import (
	"GADS/models"
	"GADS/util"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddUser(c *gin.Context) {
	var user models.User

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if user == (models.User{}) {
		BadRequest(c, "empty or invalid body")
		return
	}

	dbUser, err := util.GetUserFromDB(user.Username)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "failed checking for user in db - "+err.Error())
		return
	} else {
		fmt.Println("user does not exist, creating")
		// ADD LOGGER HERE
	}

	if dbUser != (models.User{}) {
		BadRequest(c, "user already exists")
		return
	}

	if user.Username == "" || user.Password == "" || user.Role == "" {
		BadRequest(c, "username, password and role are mandatory")
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		BadRequest(c, "invalid role - `admin` and `user` are the accepted values")
		return
	}

	err = util.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("failed adding/updating user - %s", err))
		return
	}

	OK(c, "successfully added user")
}
