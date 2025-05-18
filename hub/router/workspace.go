package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/auth"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateWorkspace(c *gin.Context) {
	var workspace models.Workspace
	if err := c.ShouldBindJSON(&workspace); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	workspace.IsDefault = false

	// Validate unique name
	existingWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace name must be unique"})
			return
		}
	}

	// Save to database
	err := db.GlobalMongoStore.AddWorkspace(&workspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

func UpdateWorkspace(c *gin.Context) {
	var workspace models.Workspace
	if err := c.ShouldBindJSON(&workspace); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Validate unique workspace name
	existingWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name && ws.ID != workspace.ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace name must be unique"})
			return
		}
	}

	// Update workspace in database
	err := db.GlobalMongoStore.UpdateWorkspace(&workspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workspace"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

func DeleteWorkspace(c *gin.Context) {
	id := c.Param("id")

	workspace, err := db.GlobalMongoStore.GetWorkspaceByID(id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workspace"})
		return
	}

	if workspace.IsDefault || db.GlobalMongoStore.WorkspaceHasDevices(id) || db.GlobalMongoStore.WorkspaceHasUsers(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default workspace or workspace with devices/users"})
		return
	}

	err = db.GlobalMongoStore.DeleteWorkspace(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workspace"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace deleted"})
}

func GetWorkspaces(c *gin.Context) {
	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	searchStr := c.Query("search")
	tenantStr := c.Query("tenant")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Default limit
	}

	workspaces, totalCount := db.GlobalMongoStore.GetWorkspacesPaginated(page, limit, searchStr)

	// Filter by tenant if specified
	if tenantStr != "" {
		var filteredWorkspaces []models.Workspace
		for _, ws := range workspaces {
			if ws.Tenant == tenantStr {
				filteredWorkspaces = append(filteredWorkspaces, ws)
			}
		}
		workspaces = filteredWorkspaces
		totalCount = int64(len(filteredWorkspaces))
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces, "total": totalCount})
}

func GetUserWorkspaces(c *gin.Context) {
	// Get JWT token from Authorization header
	authHeader := c.GetHeader("Authorization")

	var username string
	var role string
	var tenant string
	var issuer string

	if authHeader != "" {
		// Extract token from Bearer format
		tokenString, err := auth.ExtractTokenFromBearer(authHeader)
		if err == nil {
			// Get origin from request
			origin := auth.GetOriginFromRequest(c)

			// Get claims from token with origin
			claims, err := auth.GetClaimsFromToken(tokenString, origin)
			if err == nil {
				username = claims.Username
				role = claims.Role
				tenant = claims.Tenant
				issuer = claims.Issuer
			}
		}
	}

	// If we couldn't get the user info from JWT token, try the legacy session approach
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	searchStr := c.Query("search")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Default limit
	}

	var workspaces []models.Workspace

	// If user is admin, return all workspaces
	if role == "admin" {
		workspaces, _ = db.GlobalMongoStore.GetWorkspacesPaginated(page, limit, searchStr)
	} else {
		// Check if the token was issued by GADS itself
		if issuer == "gads" {
			// For internal tokens, use the standard method based on user association
			workspaces = db.GlobalMongoStore.GetUserWorkspaces(username)
		} else {
			// For external tokens, get all workspaces and filter by tenant
			allWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()

			// If tenant is specified, filter by it
			if tenant != "" {
				for _, ws := range allWorkspaces {
					if ws.Tenant == tenant {
						workspaces = append(workspaces, ws)
					}
				}
			} else {
				// If there is no tenant in the token, show only workspaces without tenant
				for _, ws := range allWorkspaces {
					if ws.Tenant == "" {
						workspaces = append(workspaces, ws)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"workspaces": workspaces,
		"total":      len(workspaces),
	})
}
