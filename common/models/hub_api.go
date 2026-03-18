package models

// Generic response types
// LegacyAPIResponse is the old untyped response wrapper — kept for backward compat during migration.
type LegacyAPIResponse struct {
	Message string      `json:"message,omitempty"`
	Result  interface{} `json:"result,omitempty"`
}

type TypedAPIResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  T      `json:"result"`
}

// Specific types
type SystemStatusAPIResponse = TypedAPIResponse[SystemStatusResponse]
type UserInfoAPIResponse = TypedAPIResponse[UserInfoResponse]
type WorkspaceInfoAPIResponse = TypedAPIResponse[Workspace]

// Types
type SystemStatusMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Action  string `json:"action"`
}

type SystemStatusResponse struct {
	Messages []SystemStatusMessage `json:"messages"`
}
