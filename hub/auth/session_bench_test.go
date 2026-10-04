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
	"sync/atomic"
	"testing"
	"time"
)

// benchmarkSessions fills the store with n sessions and returns their ids
func benchmarkSessions(b *testing.B, n int) []string {
	b.Helper()

	sessionsMu.Lock()
	previousSessions := sessions
	sessions = make(map[string]*Session, n)
	sessionsMu.Unlock()

	b.Cleanup(func() {
		sessionsMu.Lock()
		sessions = previousSessions
		sessionsMu.Unlock()
	})

	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, CreateSession("bench-user", "").ID)
	}

	return ids
}

// BenchmarkTouchSession measures the cost of the session check every
// authenticated request pays
func BenchmarkTouchSession(b *testing.B) {
	ids := benchmarkSessions(b, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := TouchSession(ids[i%len(ids)]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTouchSessionParallel measures the same under concurrent load, where
// the single mutex guarding the store is the thing that could bite
func BenchmarkTouchSessionParallel(b *testing.B) {
	ids := benchmarkSessions(b, 1000)

	var counter atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := ids[int(counter.Add(1))%len(ids)]
			if _, err := TouchSession(id); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkValidateJWT measures the token validation the session check sits
// behind, for comparison. Note this runs against an in-memory secret store -
// the real one reads the secret key from MongoDB on every call
func BenchmarkValidateJWT(b *testing.B) {
	previousCache := secretCache
	secretCache = setupTestSecretCache()
	b.Cleanup(func() {
		secretCache = previousCache
	})
	benchmarkSessions(b, 1)

	token, err := GenerateJWT("bench-user", "user", "tenant1", []string{"user"})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ValidateJWT(token); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSweepExpiredSessions measures the janitor pass, which holds the
// store's lock for its whole run
func BenchmarkSweepExpiredSessions(b *testing.B) {
	benchmarkSessions(b, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sweepExpiredSessions(time.Now())
	}
}
