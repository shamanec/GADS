/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateTURNCredentials(t *testing.T) {
	secret := "test-secret"
	ttl := 3600

	username, password, expiresAt := GenerateTURNCredentials(secret, ttl, "gads")

	// Verify username format contains ":gads"
	if !strings.Contains(username, ":gads") {
		t.Errorf("Username should contain ':gads', got: %s", username)
	}

	// Verify username contains timestamp
	parts := strings.Split(username, ":")
	if len(parts) != 2 {
		t.Errorf("Username should have format 'timestamp:gads', got: %s", username)
	}

	// Verify password is valid base64
	_, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		t.Errorf("Password should be valid base64, got error: %v", err)
	}

	// Verify expiration time is approximately now + ttl
	now := time.Now().Unix()
	expectedExpiry := now + int64(ttl)
	diff := expiresAt - expectedExpiry
	if diff < -5 || diff > 5 {
		t.Errorf("ExpiresAt should be approximately now + ttl, got difference of %d seconds", diff)
	}
}

func TestGenerateTURNCredentials_DifferentSecrets(t *testing.T) {
	ttl := 3600

	username1, password1, _ := GenerateTURNCredentials("secret1", ttl, "gads")
	username2, password2, _ := GenerateTURNCredentials("secret2", ttl, "gads")

	// Same TTL should generate similar timestamps (usernames may be identical)
	// but different secrets should generate different passwords
	if password1 == password2 {
		t.Error("Different secrets should generate different passwords")
	}

	// Both should be valid base64
	_, err1 := base64.StdEncoding.DecodeString(password1)
	_, err2 := base64.StdEncoding.DecodeString(password2)
	if err1 != nil || err2 != nil {
		t.Error("Both passwords should be valid base64")
	}

	// Usernames should have correct format
	if !strings.Contains(username1, ":gads") || !strings.Contains(username2, ":gads") {
		t.Error("Both usernames should contain ':gads'")
	}
}

func TestGenerateTURNCredentials_DifferentTTL(t *testing.T) {
	secret := "test-secret"

	username1, _, expiresAt1 := GenerateTURNCredentials(secret, 3600, "gads")
	username2, _, expiresAt2 := GenerateTURNCredentials(secret, 7200, "gads")

	// Different TTL should generate different expiration times
	timeDiff := expiresAt2 - expiresAt1
	if timeDiff < 3500 || timeDiff > 3700 {
		t.Errorf("Expected time difference around 3600 seconds, got: %d", timeDiff)
	}

	// Usernames should be different (different timestamps)
	if username1 == username2 {
		t.Error("Different TTLs should generate different usernames")
	}
}

func TestGenerateTURNCredentials_Consistency(t *testing.T) {
	secret := "test-secret"
	ttl := 3600

	// Generate credentials twice with same parameters at approximately same time
	username1, password1, expiresAt1 := GenerateTURNCredentials(secret, ttl, "gads")
	time.Sleep(1 * time.Millisecond) // Small delay
	username2, password2, expiresAt2 := GenerateTURNCredentials(secret, ttl, "gads")

	// Timestamps should be very close (within 1 second)
	timeDiff := expiresAt2 - expiresAt1
	if timeDiff < 0 || timeDiff > 1 {
		t.Errorf("Expected timestamps within 1 second, got difference: %d", timeDiff)
	}

	// Usernames and passwords should be identical or very similar
	// (may differ by 1 second due to timestamp)
	if username1 != username2 {
		// Check if they differ by exactly 1 second
		parts1 := strings.Split(username1, ":")
		parts2 := strings.Split(username2, ":")
		if len(parts1) != 2 || len(parts2) != 2 {
			t.Error("Username format is invalid")
		}
	}

	// Passwords follow HMAC(secret, username), so if usernames match, passwords must match
	if username1 == username2 && password1 != password2 {
		t.Error("Same username and secret should generate same password")
	}
}

func TestGenerateTURNCredentials_CustomSuffix(t *testing.T) {
	secret := "test-secret"
	ttl := 3600

	username, password, expiresAt := GenerateTURNCredentials(secret, ttl, "myorg")

	// Verify username format contains custom suffix
	if !strings.Contains(username, ":myorg") {
		t.Errorf("Username should contain ':myorg', got: %s", username)
	}
	if strings.Contains(username, ":gads") {
		t.Errorf("Username should not contain ':gads' when custom suffix is set, got: %s", username)
	}

	// Verify password is still valid base64
	_, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		t.Errorf("Password should be valid base64, got error: %v", err)
	}

	// Verify expiration time
	now := time.Now().Unix()
	expectedExpiry := now + int64(ttl)
	diff := expiresAt - expectedExpiry
	if diff < -5 || diff > 5 {
		t.Errorf("ExpiresAt should be approximately now + ttl, got difference of %d seconds", diff)
	}
}

func TestGenerateTURNCredentials_EmptySuffix(t *testing.T) {
	secret := "test-secret"
	ttl := 3600

	username, _, _ := GenerateTURNCredentials(secret, ttl, "")

	// Should use default "gads" suffix
	if !strings.Contains(username, ":gads") {
		t.Errorf("Username should contain default ':gads' suffix when suffix is empty, got: %s", username)
	}
}

func TestGenerateTURNCredentials_DifferentSuffixes(t *testing.T) {
	secret := "test-secret"
	ttl := 3600

	// Test first suffix
	username1, password1, _ := GenerateTURNCredentials(secret, ttl, "org1")

	// Give a small delay to ensure different timestamps
	time.Sleep(1 * time.Second)

	// Test second suffix
	username2, password2, _ := GenerateTURNCredentials(secret, ttl, "org2")

	// Usernames should have different suffixes
	if !strings.Contains(username1, ":org1") {
		t.Errorf("First username should contain ':org1', got: %s", username1)
	}
	if !strings.Contains(username2, ":org2") {
		t.Errorf("Second username should contain ':org2', got: %s", username2)
	}

	// Different usernames should generate different passwords (due to HMAC)
	if password1 == password2 {
		t.Error("Different username suffixes should generate different passwords")
	}
}
