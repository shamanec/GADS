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
	existingWorkspaces := db.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace name must be unique"})
			return
		}
	}

	// Save to database
	err := db.AddWorkspace(&workspace)
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
	existingWorkspaces := db.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name && ws.ID != workspace.ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace name must be unique"})
			return
		}
	}

	// Update workspace in database
	err := db.UpdateWorkspace(&workspace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workspace"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

func DeleteWorkspace(c *gin.Context) {
	id := c.Param("id")

	workspace, err := db.GetWorkspaceByID(id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workspace"})
		return
	}

	if workspace.IsDefault || db.WorkspaceHasDevices(id) || db.WorkspaceHasUsers(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default workspace or workspace with devices/users"})
		return
	}

	err = db.DeleteWorkspace(id)
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

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Default limit
	}

	workspaces, totalCount := db.GetWorkspacesPaginated(page, limit, searchStr)
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces, "total": totalCount})
}

func GetUserWorkspaces(c *gin.Context) {
	// Get session ID from header
	sessionID := c.GetHeader("X-Auth-Token")

	// Get user from session
	session, exists := auth.GetSession(sessionID)
	if !exists {
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
	var totalCount int64

	// If user is admin, return all workspaces
	if session.User.Role == "admin" {
		workspaces, totalCount = db.GetWorkspacesPaginated(page, limit, searchStr)
	} else {
		// For non-admin users, only return workspaces associated with the user
		workspaces, totalCount = db.GetUserWorkspacesPaginated(session.User.Username, page, limit, searchStr)
	}

	c.JSON(http.StatusOK, gin.H{
		"workspaces": workspaces,
		"total":      totalCount,
	})
}
