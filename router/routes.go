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

	if user.Username == "" {
		user.Username = "New user"
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

func GetProviders(c *gin.Context) {
	providers := util.GetProvidersFromDB()
	if len(providers) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	OkJSON(c, providers)
}

func GetProviderInfo(c *gin.Context) {
	providerName := c.Param("name")
	providers := util.GetProvidersFromDB()
	for _, provider := range providers {
		if provider.Name == providerName {
			c.JSON(http.StatusOK, provider)
			return
		}
	}
	NotFound(c, fmt.Sprintf("No provider with name `%s` found", providerName))
}

func AddProvider(c *gin.Context) {
	var provider util.ProviderDB
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if provider == (util.ProviderDB{}) {
		BadRequest(c, "Empty or invalid body")
		return
	}

	// Validations
	if provider.Nickname == "" {
		BadRequest(c, "Missing or invalid nickname")
		return
	}
	providerDB, _ := util.GetProviderFromDB(provider.Nickname)
	if (providerDB != util.ProviderDB{}) {
		BadRequest(c, "Provider with this nickname already exists")
		return
	}

	if provider.OS == "" {
		BadRequest(c, "Missing or invalid OS")
		return
	}
	if provider.HostAddress == "" {
		BadRequest(c, "Missing or invalid host address")
		return
	}
	if provider.Port == 0 {
		BadRequest(c, "Missing or invalid port")
		return
	}
	if provider.ProvideIOS {
		if provider.WdaBundleID == "" && (provider.OS == "windows" || provider.OS == "linux") {
			BadRequest(c, "Missing or invalid WebDriverAgent bundle ID")
			return
		}
		if provider.WdaRepoPath == "" && provider.OS == "macos" {
			BadRequest(c, "Missing or invalid WebDriverAgent repo path")
			return
		}
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		BadRequest(c, "Missing or invalid Selenium Grid address")
		return
	}

	err = util.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not create provider")
		return
	}

	providersDB := util.GetProvidersFromDB()
	OkJSON(c, providersDB)
}

func UpdateProvider(c *gin.Context) {
	var provider util.ProviderDB
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		InternalServerError(c, fmt.Sprintf("%s", err))
		return
	}

	err = json.Unmarshal(body, &provider)
	if err != nil {
		BadRequest(c, fmt.Sprintf("%s", err))
		return
	}

	if provider == (util.ProviderDB{}) {
		BadRequest(c, "Empty or invalid body")
		return
	}

	// Validations
	if provider.Nickname == "" {
		BadRequest(c, "missing `nickname` field")
		return
	}
	if provider.OS == "" {
		BadRequest(c, "missing `os` field")
		return
	}
	if provider.HostAddress == "" {
		BadRequest(c, "missing `host_address` field")
		return
	}
	if provider.Port == 0 {
		BadRequest(c, "missing `port` field")
		return
	}
	if provider.ProvideIOS {
		if provider.WdaBundleID == "" && (provider.OS == "windows" || provider.OS == "linux") {
			BadRequest(c, "missing `wda_bundle_id` field")
			return
		}
		if provider.WdaRepoPath == "" && provider.OS == "macos" {
			BadRequest(c, "missing `wda_repo_path` field")
			return
		}
	}
	if provider.UseSeleniumGrid && provider.SeleniumGrid == "" {
		BadRequest(c, "missing `selenium_grid` field")
		return
	}

	err = util.AddOrUpdateProvider(provider)
	if err != nil {
		InternalServerError(c, "Could not update provider")
		return
	}
	OK(c, "Provider updated successfully")
}
