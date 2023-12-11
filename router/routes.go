package router

import (
	"GADS/models"
	"GADS/util"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

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
		BadRequest(c, "Empty or invalid body")
		return
	}

	if user.Email == "" || user.Password == "" || user.Role == "" {
		BadRequest(c, "Email, password and role are mandatory")
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		BadRequest(c, "Invalid role - `admin` and `user` are the accepted values")
		return
	}

	dbUser, err := util.GetUserFromDB(user.Email)
	if err != nil && err != mongo.ErrNoDocuments {
		InternalServerError(c, "Failed checking for user in db - "+err.Error())
		return
	} else {
		fmt.Println("User does not exist, creating")
		// ADD LOGGER HERE
	}

	if dbUser != (models.User{}) {
		BadRequest(c, "User already exists")
		return
	}

	err = util.AddOrUpdateUser(user)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("Failed adding/updating user - %s", err))
		return
	}

	OK(c, "Successfully added user")
}
