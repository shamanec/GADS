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
