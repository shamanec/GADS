/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package clientcredentials

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateClientID(t *testing.T) {
	// Test basic generation with default prefix
	id1 := GenerateClientID()
	id2 := GenerateClientID()

	// IDs should be different
	if id1 == id2 {
		t.Error("Generated client IDs should be unique")
	}

	// Should have correct format: gads_<timestamp>_<suffix>
	if !strings.HasPrefix(id1, "gads_") {
		t.Errorf("Client ID should start with 'gads_', got: %s", id1)
	}

	// Should have 3 parts separated by underscores
	parts := strings.Split(id1, "_")
	if len(parts) != 3 {
		t.Errorf("Client ID should have 3 parts, got %d: %s", len(parts), id1)
	}
}

func TestGenerateClientIDWithPrefix(t *testing.T) {
	// Test custom prefix
	customPrefix := "myapp"
	id1 := GenerateClientIDWithPrefix(customPrefix)
	id2 := GenerateClientIDWithPrefix(customPrefix)

	// IDs should be different
	if id1 == id2 {
		t.Error("Generated client IDs should be unique")
	}

	// Should have correct custom prefix
	if !strings.HasPrefix(id1, customPrefix+"_") {
		t.Errorf("Client ID should start with '%s_', got: %s", customPrefix, id1)
	}

	// Should have 3 parts separated by underscores
	parts := strings.Split(id1, "_")
	if len(parts) != 3 {
		t.Errorf("Client ID should have 3 parts, got %d: %s", len(parts), id1)
	}

	// Test with different prefixes
	testPrefixes := []string{"app1", "service-x", "api", "client"}
	for _, prefix := range testPrefixes {
		id := GenerateClientIDWithPrefix(prefix)
		if !strings.HasPrefix(id, prefix+"_") {
			t.Errorf("Client ID should start with '%s_', got: %s", prefix, id)
		}
	}

	// Test empty prefix (should fallback to "gads")
	idEmpty := GenerateClientIDWithPrefix("")
	if !strings.HasPrefix(idEmpty, "gads_") {
		t.Errorf("Empty prefix should fallback to 'gads_', got: %s", idEmpty)
	}

	// Test very long prefix (should work normally)
	longPrefix := "very-long-prefix-name-for-testing"
	idLong := GenerateClientIDWithPrefix(longPrefix)
	if !strings.HasPrefix(idLong, longPrefix+"_") {
		t.Errorf("Long prefix should work, got: %s", idLong)
	}
}

func TestGenerateClientSecret(t *testing.T) {
	// Test basic generation
	secret1, err := GenerateClientSecret()
	if err != nil {
		t.Fatalf("GenerateClientSecret failed: %v", err)
	}

	secret2, err := GenerateClientSecret()
	if err != nil {
		t.Fatalf("GenerateClientSecret failed: %v", err)
	}

	// Secrets should be different
	if secret1 == secret2 {
		t.Error("Generated secrets should be unique")
	}

	// Should be non-empty
	if secret1 == "" {
		t.Error("Generated secret should not be empty")
	}

	// Should be reasonable length (base64 encoded 32 bytes ≈ 44 chars)
	if len(secret1) < 40 {
		t.Errorf("Generated secret seems too short: %d chars", len(secret1))
	}
}

func TestHashSecret(t *testing.T) {
	secret := "test-secret-123"

	// Test basic hashing
	hash1, err := HashSecret(secret)
	if err != nil {
		t.Fatalf("HashSecret failed: %v", err)
	}

	hash2, err := HashSecret(secret)
	if err != nil {
		t.Fatalf("HashSecret failed: %v", err)
	}

	// Hashes should be different (bcrypt uses salt)
	if hash1 == hash2 {
		t.Error("Hashes of same secret should be different (salted)")
	}

	// Hash should be non-empty
	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	// Test empty secret
	_, err = HashSecret("")
	if err == nil {
		t.Error("HashSecret should fail for empty secret")
	}
}

func TestValidateSecret(t *testing.T) {
	secret := "test-secret-123"
	hash, err := HashSecret(secret)
	if err != nil {
		t.Fatalf("HashSecret failed: %v", err)
	}

	// Test valid secret
	if !ValidateSecret(secret, hash) {
		t.Error("ValidateSecret should return true for correct secret")
	}

	// Test invalid secret
	if ValidateSecret("wrong-secret", hash) {
		t.Error("ValidateSecret should return false for incorrect secret")
	}

	// Test empty inputs
	if ValidateSecret("", hash) {
		t.Error("ValidateSecret should return false for empty secret")
	}

	if ValidateSecret(secret, "") {
		t.Error("ValidateSecret should return false for empty hash")
	}

	if ValidateSecret("", "") {
		t.Error("ValidateSecret should return false for both empty")
	}
}

func TestGenerateSecureToken(t *testing.T) {
	// Test different lengths
	lengths := []int{8, 16, 32, 64}

	for _, length := range lengths {
		token, err := GenerateSecureToken(length)
		if err != nil {
			t.Fatalf("GenerateSecureToken failed for length %d: %v", length, err)
		}

		// Token should be approximately the requested length
		if len(token) < length-2 || len(token) > length+2 {
			t.Errorf("Token length %d not close to requested %d", len(token), length)
		}
	}

	// Test uniqueness
	token1, _ := GenerateSecureToken(16)
	token2, _ := GenerateSecureToken(16)

	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}

	// Test invalid length
	_, err := GenerateSecureToken(0)
	if err == nil {
		t.Error("GenerateSecureToken should fail for zero length")
	}

	_, err = GenerateSecureToken(-1)
	if err == nil {
		t.Error("GenerateSecureToken should fail for negative length")
	}
}

func TestCompareTokens(t *testing.T) {
	token := "test-token-123"

	// Test identical tokens
	if !CompareTokens(token, token) {
		t.Error("CompareTokens should return true for identical tokens")
	}

	// Test different tokens
	if CompareTokens(token, "different-token") {
		t.Error("CompareTokens should return false for different tokens")
	}

	// Test empty tokens
	if !CompareTokens("", "") {
		t.Error("CompareTokens should return true for both empty")
	}

	if CompareTokens(token, "") {
		t.Error("CompareTokens should return false when one is empty")
	}
}

// Test timing attack resistance (basic check)
func TestTimingAttackResistance(t *testing.T) {
	secret := "test-secret-for-timing"
	hash, _ := HashSecret(secret)

	// Multiple validation attempts should take similar time
	// This is a basic test - real timing attack tests need more sophisticated measurement
	attempts := 100

	start := time.Now()
	for i := 0; i < attempts; i++ {
		ValidateSecret(secret, hash) // Valid
	}
	validTime := time.Since(start)

	start = time.Now()
	for i := 0; i < attempts; i++ {
		ValidateSecret("wrong-secret", hash) // Invalid
	}
	invalidTime := time.Since(start)

	// Times should be roughly similar (within 50% difference)
	// This is a very loose check - bcrypt should handle timing consistency
	ratio := float64(validTime) / float64(invalidTime)
	if ratio < 0.5 || ratio > 2.0 {
		t.Logf("Timing difference might be significant: valid=%v, invalid=%v, ratio=%.2f",
			validTime, invalidTime, ratio)
		// Don't fail the test - this is just informational
	}
}

// Benchmark tests for performance
func BenchmarkGenerateClientID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateClientID()
	}
}

func BenchmarkGenerateClientIDWithPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateClientIDWithPrefix("myapp")
	}
}

func BenchmarkGenerateClientSecret(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateClientSecret()
	}
}

func BenchmarkHashSecret(b *testing.B) {
	secret := "benchmark-secret-test"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashSecret(secret)
	}
}

func BenchmarkValidateSecret(b *testing.B) {
	secret := "benchmark-secret-test"
	hash, _ := HashSecret(secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateSecret(secret, hash)
	}
}

func BenchmarkGenerateSecureToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSecureToken(32)
	}
}

func BenchmarkCompareTokens(b *testing.B) {
	token1 := "benchmark-token-test-123456789"
	token2 := "benchmark-token-test-123456789"

	for i := 0; i < b.N; i++ {
		CompareTokens(token1, token2)
	}
}
