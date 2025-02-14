package utils

import "strings"

// FormatWorkspaceID formats the workspace name to a valid workspace ID.
func FormatWorkspaceID(workspaceName string) string {
	// Convert to lowercase and replace invalid characters
	workspaceID := strings.ToLower(workspaceName)
	workspaceID = strings.ReplaceAll(workspaceID, " ", "-") // Replace spaces with hyphens
	// Add more replacements as needed for other invalid characters
	return workspaceID
}
