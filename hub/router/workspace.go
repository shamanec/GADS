/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

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

// CreateWorkspace godoc
// @Summary      Create a new workspace
// @Description  Create a new workspace in the system
// @Tags         Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        workspace  body      models.Workspace  true  "Workspace data"
// @Success      200        {object}  models.Workspace
// @Failure      400        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/workspaces [post]
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

// UpdateWorkspace godoc
// @Summary      Update a workspace
// @Description  Update an existing workspace in the system
// @Tags         Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        workspace  body      models.Workspace  true  "Workspace data"
// @Success      200        {object}  models.Workspace
// @Failure      400        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/workspaces [put]
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

// DeleteWorkspace godoc
// @Summary      Delete a workspace
// @Description  Remove a workspace from the system
// @Tags         Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "Workspace ID"
// @Success      200 {object}  models.SuccessResponse
// @Failure      400 {object}  models.ErrorResponse
// @Failure      404 {object}  models.ErrorResponse
// @Failure      500 {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/workspaces/{id} [delete]
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

// GetWorkspaces godoc
// @Summary      Get all workspaces
// @Description  Retrieve list of all workspaces with pagination and filtering
// @Tags         Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        page   query  int     false  "Page number (default 1)"
// @Param        limit  query  int     false  "Items per page (default 10)"
// @Param        search query  string  false  "Search term"
// @Param        tenant query  string  false  "Filter by tenant"
// @Success      200    {object}  models.WorkspacesResponse
// @Security     BearerAuth
// @Router       /admin/workspaces [get]
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

// GetUserWorkspaces godoc
// @Summary      Get user workspaces
// @Description  Retrieve workspaces accessible to the current user
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        page   query  int     false  "Page number (default 1)"
// @Param        limit  query  int     false  "Items per page (default 10)"
// @Param        search query  string  false  "Search term"
// @Success      200    {object}  models.WorkspacesResponse
// @Failure      401    {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /workspaces [get]
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
