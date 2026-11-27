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
	"net/url"
	"os"
	"strings"
	"testing"
)

func clearMongoCredentialsEnv(t *testing.T) {
	t.Helper()
	t.Setenv(MongoUserEnv, "")
	t.Setenv(MongoPasswordEnv, "")
}

func TestBuildConnectionStringNoCredentials(t *testing.T) {
	clearMongoCredentialsEnv(t)

	uri, err := BuildConnectionString("localhost:27017")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if uri != "mongodb://localhost:27017/?keepAlive=true" {
		t.Fatalf("unexpected URI: %s", uri)
	}
}

func TestBuildConnectionStringEnvCredentials(t *testing.T) {
	t.Setenv(MongoUserEnv, "gadsuser")
	t.Setenv(MongoPasswordEnv, "secret")

	uri, err := BuildConnectionString("192.168.1.6:27017")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("result is not a valid URI: %s", err)
	}
	if parsed.User == nil {
		t.Fatal("expected credentials in the URI")
	}
	if parsed.User.Username() != "gadsuser" {
		t.Fatalf("unexpected username: %s", parsed.User.Username())
	}
	password, _ := parsed.User.Password()
	if password != "secret" {
		t.Fatalf("unexpected password: %s", password)
	}
	if parsed.Host != "192.168.1.6:27017" {
		t.Fatalf("unexpected host: %s", parsed.Host)
	}
	if parsed.Query().Get("keepAlive") != "true" {
		t.Fatalf("keepAlive option missing: %s", uri)
	}
}

func TestBuildConnectionStringSpecialCharacters(t *testing.T) {
	t.Setenv(MongoUserEnv, "gads user")
	t.Setenv(MongoPasswordEnv, "p@ss:word/%25?x")

	uri, err := BuildConnectionString("localhost:27017")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// The raw password must not appear verbatim in the connection string
	if strings.Contains(uri, "p@ss:word/%25?x") {
		t.Fatalf("password was not escaped: %s", uri)
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("result is not a valid URI: %s", err)
	}
	if parsed.User.Username() != "gads user" {
		t.Fatalf("username did not round-trip: %s", parsed.User.Username())
	}
	password, _ := parsed.User.Password()
	if password != "p@ss:word/%25?x" {
		t.Fatalf("password did not round-trip: %s", password)
	}
}

func TestBuildConnectionStringPartialCredentials(t *testing.T) {
	t.Setenv(MongoUserEnv, "gadsuser")
	t.Setenv(MongoPasswordEnv, "")
	if _, err := BuildConnectionString("localhost:27017"); err == nil {
		t.Fatal("expected an error when only the user is set")
	}

	t.Setenv(MongoUserEnv, "")
	t.Setenv(MongoPasswordEnv, "secret")
	if _, err := BuildConnectionString("localhost:27017"); err == nil {
		t.Fatal("expected an error when only the password is set")
	}
}

func TestBuildConnectionStringEmbeddedCredentials(t *testing.T) {
	clearMongoCredentialsEnv(t)

	uri, err := BuildConnectionString("gadsuser:secret@localhost:27017")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("result is not a valid URI: %s", err)
	}
	if parsed.User == nil || parsed.User.Username() != "gadsuser" {
		t.Fatalf("embedded credentials were not preserved: %s", uri)
	}
	if parsed.Query().Get("keepAlive") != "true" {
		t.Fatalf("keepAlive option missing: %s", uri)
	}
}

func TestBuildConnectionStringConflictingCredentials(t *testing.T) {
	t.Setenv(MongoUserEnv, "gadsuser")
	t.Setenv(MongoPasswordEnv, "secret")

	if _, err := BuildConnectionString("otheruser:otherpass@localhost:27017"); err == nil {
		t.Fatal("expected an error when credentials are provided both ways")
	}
}

func TestBuildConnectionStringAuthDatabasePath(t *testing.T) {
	t.Setenv(MongoUserEnv, "gadsuser")
	t.Setenv(MongoPasswordEnv, "secret")

	uri, err := BuildConnectionString("localhost:27017/gads")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("result is not a valid URI: %s", err)
	}
	if parsed.Path != "/gads" {
		t.Fatalf("auth database path was not preserved: %s", uri)
	}
}

func TestBuildConnectionStringExistingQueryParams(t *testing.T) {
	clearMongoCredentialsEnv(t)

	uri, err := BuildConnectionString("localhost:27017/?authSource=admin")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("result is not a valid URI: %s", err)
	}
	if parsed.Query().Get("authSource") != "admin" {
		t.Fatalf("existing query parameter was lost: %s", uri)
	}
	if parsed.Query().Get("keepAlive") != "true" {
		t.Fatalf("keepAlive option missing: %s", uri)
	}
}

func TestBuildConnectionStringInvalidAddress(t *testing.T) {
	clearMongoCredentialsEnv(t)

	if _, err := BuildConnectionString(""); err == nil {
		t.Fatal("expected an error for an empty address")
	}
}

func TestBuildConnectionStringErrorDoesNotLeakPassword(t *testing.T) {
	clearMongoCredentialsEnv(t)

	// Invalid port makes url.Parse fail; the error must not contain the password
	_, err := BuildConnectionString("gadsuser:supersecret@localhost:badport")
	if err == nil {
		t.Fatal("expected an error for an invalid address")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("error message leaks the password: %s", err)
	}
}

func TestRedactMongoURI(t *testing.T) {
	redacted := RedactMongoURI("mongodb://gadsuser:supersecret@localhost:27017/?keepAlive=true")
	if strings.Contains(redacted, "supersecret") {
		t.Fatalf("password was not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "redacted") {
		t.Fatalf("expected redaction marker: %s", redacted)
	}
	if !strings.Contains(redacted, "localhost:27017") {
		t.Fatalf("host should be preserved: %s", redacted)
	}
}

func TestRedactMongoURINoCredentials(t *testing.T) {
	uri := "mongodb://localhost:27017/?keepAlive=true"
	if RedactMongoURI(uri) != uri {
		t.Fatal("URI without credentials should be returned unchanged")
	}
}

func TestRedactMongoURIInvalid(t *testing.T) {
	redacted := RedactMongoURI("mongodb://gadsuser:supersecret@localhost:badport")
	if strings.Contains(redacted, "supersecret") {
		t.Fatalf("invalid URI leaks the password: %s", redacted)
	}
}

func TestRedactMongoURIUsernameOnly(t *testing.T) {
	redacted := RedactMongoURI("mongodb://gadsuser@localhost:27017")
	if strings.Contains(redacted, "gadsuser") {
		t.Fatalf("username was not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "localhost:27017") {
		t.Fatalf("host should be preserved: %s", redacted)
	}
}

func TestRedactMongoURIMalformedWithAtInQuery(t *testing.T) {
	// An `@` inside the query must not swallow the host during fallback masking
	redacted := RedactMongoURI("mongodb://gadsuser:supersecret@localhost:badport/?appName=a@b")
	if strings.Contains(redacted, "supersecret") {
		t.Fatalf("password leaks: %s", redacted)
	}
	if !strings.Contains(redacted, "localhost:badport") {
		t.Fatalf("host was swallowed by the query `@`: %s", redacted)
	}
	if !strings.Contains(redacted, "appName=a@b") {
		t.Fatalf("query should be preserved: %s", redacted)
	}
}

func TestRedactMongoURINoScheme(t *testing.T) {
	redacted := RedactMongoURI("gadsuser:supersecret@localhost:27017")
	if strings.Contains(redacted, "supersecret") {
		t.Fatalf("scheme-less string leaks the password: %s", redacted)
	}
	if !strings.Contains(redacted, "localhost:27017") {
		t.Fatalf("host should be preserved: %s", redacted)
	}
}

func TestRedactMongoURINoSchemeNoCredentials(t *testing.T) {
	if RedactMongoURI("localhost:27017") != "localhost:27017" {
		t.Fatal("plain address without credentials should be returned unchanged")
	}
}

func TestClearMongoCredentialsFromEnv(t *testing.T) {
	t.Setenv(MongoUserEnv, "gadsuser")
	t.Setenv(MongoPasswordEnv, "secret")

	ClearMongoCredentialsFromEnv()

	if os.Getenv(MongoUserEnv) != "" {
		t.Fatalf("%s was not removed from the environment", MongoUserEnv)
	}
	if os.Getenv(MongoPasswordEnv) != "" {
		t.Fatalf("%s was not removed from the environment", MongoPasswordEnv)
	}
}