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
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"GADS/hub/auth"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// ensureWorkspaceTenant ensures the workspace has a tenant, using default if empty
func ensureWorkspaceTenant(workspace *models.Workspace, c *gin.Context) error {
	if workspace.Tenant == "" {
		defaultTenant, err := db.GlobalMongoStore.GetOrCreateDefaultTenant()
		if err != nil {
			api.InternalError(c, "Failed to get default tenant")
			return err
		}
		workspace.Tenant = defaultTenant
	}
	return nil
}

// CreateWorkspace godoc
// @Summary      Create a new workspace
// @Description  Create a new workspace in the system
// @Tags         Hub - Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        workspace  body      models.Workspace  true  "Workspace data"
// @Success      200        {object}  models.WorkspaceResponse
// @Failure      400        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/workspaces [post]
func CreateWorkspace(c *gin.Context) {
	var workspace models.Workspace
	if err := c.ShouldBindJSON(&workspace); err != nil {
		api.BadRequest(c, "Invalid input")
		return
	}

	workspace.IsDefault = false

	if err := ensureWorkspaceTenant(&workspace, c); err != nil {
		return
	}

	// Validate unique name
	existingWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name {
			api.BadRequest(c, "Workspace name must be unique")
			return
		}
	}

	// Save to database
	err := db.GlobalMongoStore.AddWorkspace(&workspace)
	if err != nil {
		api.InternalError(c, "Failed to create workspace")
		return
	}

	api.OK(c, "", workspace)
}

// UpdateWorkspace godoc
// @Summary      Update a workspace
// @Description  Update an existing workspace in the system
// @Tags         Hub - Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        workspace  body      models.Workspace  true  "Workspace data"
// @Success      200        {object}  models.WorkspaceResponse
// @Failure      400        {object}  models.ErrorResponse
// @Failure      500        {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/workspaces [put]
func UpdateWorkspace(c *gin.Context) {
	var workspace models.Workspace
	if err := c.ShouldBindJSON(&workspace); err != nil {
		api.BadRequest(c, "Invalid input")
		return
	}

	if err := ensureWorkspaceTenant(&workspace, c); err != nil {
		return
	}

	// Validate unique workspace name
	existingWorkspaces, _ := db.GlobalMongoStore.GetWorkspaces()
	for _, ws := range existingWorkspaces {
		if ws.Name == workspace.Name && ws.ID != workspace.ID {
			api.BadRequest(c, "Workspace name must be unique")
			return
		}
	}

	// Update workspace in database
	err := db.GlobalMongoStore.UpdateWorkspace(&workspace)
	if err != nil {
		api.InternalError(c, "Failed to update workspace")
		return
	}

	api.OK(c, "", workspace)
}

// DeleteWorkspace godoc
// @Summary      Delete a workspace
// @Description  Remove a workspace from the system
// @Tags         Hub - Admin - Workspaces
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
			api.NotFound(c, "Workspace not found")
			return
		}
		api.InternalError(c, "Failed to get workspace")
		return
	}

	if workspace.IsDefault || db.GlobalMongoStore.WorkspaceHasDevices(id) || db.GlobalMongoStore.WorkspaceHasUsers(id) {
		api.BadRequest(c, "Cannot delete default workspace or workspace with devices/users")
		return
	}

	err = db.GlobalMongoStore.DeleteWorkspace(id)
	if err != nil {
		api.InternalError(c, "Failed to delete workspace")
		return
	}

	api.OKMessage(c, "Workspace deleted")
}

// GetWorkspaces godoc
// @Summary      Get all workspaces
// @Description  Retrieve list of all workspaces with pagination and filtering
// @Tags         Hub - Admin - Workspaces
// @Accept       json
// @Produce      json
// @Param        page   query  int     false  "Page number (default 1)"
// @Param        limit  query  int     false  "Items per page (default 10)"
// @Param        search query  string  false  "Search term"
// @Param        tenant query  string  false  "Filter by tenant"
// @Success      200    {object}  models.WorkspacePageResponse
// @Failure      400    {object}  models.ErrorResponse
// @Failure      500    {object}  models.ErrorResponse
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
		limit = 10
	}

	workspaces, totalCount, err := db.GlobalMongoStore.GetWorkspacesWithDeviceCount(page, limit, searchStr, tenantStr)

	if err != nil {
		if err == db.ErrInvalidPagination {
			api.BadRequest(c, "Invalid pagination parameters")
		} else {
			api.InternalError(c, "Failed to get workspaces")
		}
		return
	}

	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))
	api.OK(c, "", models.WorkspacesPage{
		Items:      workspaces,
		Total:      totalCount,
		Page:       page,
		TotalPages: totalPages,
	})
}

// GetUserWorkspaces godoc
// @Summary      Get user workspaces
// @Description  Retrieve workspaces accessible to the current user
// @Tags         Hub - Workspaces
// @Accept       json
// @Produce      json
// @Param        page   query  int     false  "Page number (default 1)"
// @Param        limit  query  int     false  "Items per page (default 10)"
// @Param        search query  string  false  "Search term"
// @Success      200    {object}  models.UserListResponse
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
		api.Unauthorized(c, "unauthorized")
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
		limit = 10
	}

	var workspaces []models.Workspace = make([]models.Workspace, 0)

	// If user is admin, return all workspaces
	if role == "admin" {
		workspaces, _ = db.GlobalMongoStore.GetWorkspacesPaginated(page, limit, searchStr)
	} else {
		// Check if the token was issued by GADS itself
		if issuer == "gads" {
			// For internal tokens, use the standard method based on user association
			workspaces = db.GlobalMongoStore.GetUserWorkspaces(username)
			if workspaces == nil {
				workspaces = make([]models.Workspace, 0)
			}
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

	api.OK(c, "", workspaces)
}
