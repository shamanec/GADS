package utils

import "strings"

func StringContainsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func StringStartsWithAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.HasPrefix(s, sub) {
			return true
		}
	}
	return false
}

func StringDoesNotContainAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// GetFloat extracts a float64 value from a parameter map with a default fallback
func GetFloat(params map[string]any, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return defaultVal
}

// GetString extracts a string value from a parameter map with a default fallback
func GetString(params map[string]any, key string, defaultVal string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}
