package router

import (
	"GADS/common/db"
	"GADS/common/models"
	"GADS/common/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateWorkspace(c *gin.Context) {
	var workspace models.Workspace
	if err := c.ShouldBindJSON(&workspace); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	workspace.ID = utils.FormatWorkspaceID(workspace.Name)

	// Validate unique ID
	existingWorkspaces := db.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.ID == workspace.ID {
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

	// Check if workspace is default or has devices
	if id == "default" || db.WorkspaceHasDevices(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default workspace or workspace with devices"})
		return
	}

	err := db.DeleteWorkspace(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workspace"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace deleted"})
}

func GetWorkspaces(c *gin.Context) {
	workspaces := db.GetWorkspaces()
	c.JSON(http.StatusOK, workspaces)
}
