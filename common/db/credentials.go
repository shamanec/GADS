/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package db

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// Environment variables holding MongoDB credentials.
// Keeping them out of the --mongo-db flag means they do not show up
// in the process command line, shell history or service arguments.
const (
	MongoUserEnv     = "GADS_MONGO_USER"
	MongoPasswordEnv = "GADS_MONGO_PASSWORD"
)

// BuildConnectionString builds a MongoDB connection string from the address
// provided via --mongo-db (host:port, optionally with an auth database path,
// e.g. host:27017/admin) and credentials from the GADS_MONGO_USER and
// GADS_MONGO_PASSWORD environment variables. Special characters in the
// credentials are escaped automatically.
//
// Credentials already embedded in the address (user:pass@host:port) keep
// working for backward compatibility, but mixing both forms is rejected.
func BuildConnectionString(dbAddress string) (string, error) {
	user := os.Getenv(MongoUserEnv)
	password := os.Getenv(MongoPasswordEnv)

	if (user == "") != (password == "") {
		return "", fmt.Errorf("both %s and %s must be set to authenticate against MongoDB", MongoUserEnv, MongoPasswordEnv)
	}

	parsed, err := url.Parse("mongodb://" + dbAddress)
	if err != nil {
		// NB: the raw parse error embeds the full URI including credentials,
		// so only the redacted address is safe to include in the message
		return "", fmt.Errorf("invalid MongoDB address `%s`", RedactMongoURI("mongodb://"+dbAddress))
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid MongoDB address `%s` - no host provided", RedactMongoURI("mongodb://"+dbAddress))
	}

	if parsed.User != nil {
		if user != "" {
			return "", fmt.Errorf("MongoDB credentials are provided both in the --mongo-db address and via %s/%s - use only one of these ways", MongoUserEnv, MongoPasswordEnv)
		}
		slog.Warn("MongoDB credentials embedded in the --mongo-db address are deprecated and expose the password in the process command line - use the "+MongoUserEnv+" and "+MongoPasswordEnv+" environment variables instead",
			"address", RedactMongoURI("mongodb://"+dbAddress))
	} else if user != "" {
		parsed.User = url.UserPassword(user, password)
	}

	// Preserve the historical keepAlive option, merging it with any
	// query parameters already present in the address
	query := parsed.Query()
	query.Set("keepAlive", "true")
	parsed.RawQuery = query.Encode()
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed.String(), nil
}

// ClearMongoCredentialsFromEnv removes the MongoDB credential environment
// variables from the current process. It is called right after the connection
// string is built so that child processes spawned later (Appium, adb, npm,
// ffmpeg, etc.) do not inherit the credentials.
func ClearMongoCredentialsFromEnv() {
	os.Unsetenv(MongoUserEnv)
	os.Unsetenv(MongoPasswordEnv)
}

// RedactMongoURI returns a MongoDB connection string with any credentials
// replaced by `redacted` so it is safe to write to logs. Strings that cannot
// be parsed (or do not look like MongoDB URIs) are masked manually to avoid
// leaking a raw password.
func RedactMongoURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err == nil && (parsed.Scheme == "mongodb" || parsed.Scheme == "mongodb+srv") {
		if parsed.User == nil {
			return uri
		}
		parsed.User = url.User("redacted")
		return parsed.String()
	}

	// Fallback for malformed or scheme-less strings - mask everything up to
	// the last `@` of the authority part so credentials never reach the logs.
	// The authority ends at the first '/', '?' or '#', so an `@` inside the
	// path or query does not swallow the host.
	prefix := ""
	rest := uri
	if schemeEnd := strings.Index(uri, "://"); schemeEnd != -1 {
		prefix = uri[:schemeEnd+3]
		rest = uri[schemeEnd+3:]
	}
	authority := rest
	if end := strings.IndexAny(rest, "/?#"); end != -1 {
		authority = rest[:end]
	}
	if at := strings.LastIndex(authority, "@"); at != -1 {
		return prefix + "redacted@" + authority[at+1:] + rest[len(authority):]
	}
	if err != nil {
		return "<invalid MongoDB URI>"
	}
	return uri
}