package auth

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AdminSecretKeyHistoryHandler returns the audit history
func AdminSecretKeyHistoryHandler(secretStore *SecretStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authentication verification is assumed to happen in previous middleware

		// Extract pagination parameters
		page := extractIntParam(r, "page", 1)
		limit := extractIntParam(r, "limit", 10)

		// Prepare filters
		filters := make(map[string]interface{})

		// Add optional filters if present
		if origin := r.URL.Query().Get("origin"); origin != "" {
			filters["origin"] = origin
		}

		if action := r.URL.Query().Get("action"); action != "" {
			filters["action"] = action
		}

		if userID := r.URL.Query().Get("user_id"); userID != "" {
			filters["user_id"] = userID
		}

		// Date filters
		if fromDateStr := r.URL.Query().Get("from_date"); fromDateStr != "" {
			if fromDate, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
				filters["from_date"] = fromDate
			}
		}

		if toDateStr := r.URL.Query().Get("to_date"); toDateStr != "" {
			if toDate, err := time.Parse(time.RFC3339, toDateStr); err == nil {
				filters["to_date"] = toDate
			}
		}

		// Get history
		auditStore := secretStore.GetSecretKeyAuditStore()
		logs, total, err := auditStore.GetHistory(page, limit, filters)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve audit history")
			return
		}

		// Format response
		response := FormatHistoryResponse(logs, total, page, limit)
		respondWithJSON(w, http.StatusOK, response)
	}
}

// AdminSecretKeyHistoryByIDHandler returns a specific audit record
func AdminSecretKeyHistoryByIDHandler(secretStore *SecretStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract ID from URL
		idStr := getURLParam(r, "id")
		if idStr == "" {
			respondWithError(w, http.StatusBadRequest, "Missing log ID")
			return
		}

		// Convert to ObjectID
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid log ID format")
			return
		}

		// Get audit log
		auditStore := secretStore.GetSecretKeyAuditStore()
		log, err := auditStore.GetAuditLogByID(id)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				respondWithError(w, http.StatusNotFound, "Audit log not found")
			} else {
				respondWithError(w, http.StatusInternalServerError, "Failed to retrieve audit log")
			}
			return
		}

		// Respond with the log
		respondWithJSON(w, http.StatusOK, log)
	}
}

// Helper functions

// extractIntParam extracts an integer parameter from the query string
func extractIntParam(r *http.Request, name string, defaultValue int) int {
	valueStr := r.URL.Query().Get(name)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value < 1 {
		return defaultValue
	}

	return value
}

// getURLParam gets a parameter from the URL (assuming a router that puts parameters in context)
func getURLParam(r *http.Request, name string) string {
	// This function should be adapted according to the router being used
	// For Gin, parameters are extracted from the context
	vars, exists := r.Context().Value("urlParams").(map[string]string)
	if !exists || vars == nil {
		// If not in context, we try to get it from Gin (if being used)
		// We check if the context has the parameter in the format Gin uses
		path := r.URL.Path
		// Simplified implementation: if there's something after the last /, we assume it's the ID
		// This is an approximation for when we can't directly access the Gin context
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return ""
	}
	return vars[name]
}

// respondWithError sends an error response to the client
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response to the client
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	// Use your application's JSON encoder here
	// Simple example:
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		// Handle encoding error
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}
