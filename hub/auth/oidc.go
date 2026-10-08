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
	"GADS/common/models"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// oidcHTTPTimeout bounds every request to the provider - discovery, token
// exchange and fetching the signing keys - so a provider that accepts the
// connection but never answers cannot hang a sign-in or the hub start
const oidcHTTPTimeout = 10 * time.Second

// The provider is discovered in the background and retried with a growing
// delay, so the hub starts without it and SSO comes up once it answers
const (
	oidcRetryMinDelay = 5 * time.Second
	oidcRetryMaxDelay = time.Minute
)

var errOIDCNotReady = errors.New("OIDC sign-in is not available")

// oidcClient is everything a sign-in needs, built from one OIDC configuration
type oidcClient struct {
	config     models.OIDCConfig
	oauth2     *oauth2.Config
	verifier   *oidc.IDTokenVerifier
	httpClient *http.Client
}

var (
	oidcMu sync.RWMutex
	// oidcCurrent is nil while SSO is off or the provider was not reached yet
	oidcCurrent *oidcClient
	// oidcGeneration grows with every configuration change, so a discovery
	// still retrying an older configuration drops its result
	oidcGeneration int
)

// ConfigureOIDC applies an OIDC configuration. A disabled configuration turns
// SSO off at once, an enabled one is discovered in the background
func ConfigureOIDC(config models.OIDCConfig) {
	oidcMu.Lock()
	oidcGeneration++
	generation := oidcGeneration
	oidcCurrent = nil
	oidcMu.Unlock()

	if !config.Enabled {
		return
	}
	go discoverOIDCProvider(config, generation)
}

// IsOIDCEnabled reports whether users can sign in through the OIDC provider
func IsOIDCEnabled() bool {
	oidcMu.RLock()
	defer oidcMu.RUnlock()
	return oidcCurrent != nil
}

func currentOIDCClient() (*oidcClient, error) {
	oidcMu.RLock()
	defer oidcMu.RUnlock()
	if oidcCurrent == nil {
		return nil, errOIDCNotReady
	}
	return oidcCurrent, nil
}

func discoverOIDCProvider(config models.OIDCConfig, generation int) {
	delay := oidcRetryMinDelay
	for {
		client, err := newOIDCClient(config)

		oidcMu.Lock()
		if generation != oidcGeneration {
			oidcMu.Unlock()
			return
		}
		if err == nil {
			oidcCurrent = client
			oidcMu.Unlock()
			slog.Info(fmt.Sprintf("OIDC sign-in enabled with issuer `%s`", config.IssuerURL))
			return
		}
		oidcMu.Unlock()

		slog.Warn(fmt.Sprintf("Failed to reach the OIDC provider `%s`, retrying in %s - %s", config.IssuerURL, delay, err))
		time.Sleep(delay)
		delay = min(delay*2, oidcRetryMaxDelay)
	}
}

func newOIDCClient(config models.OIDCConfig) (*oidcClient, error) {
	httpClient := &http.Client{Timeout: oidcHTTPTimeout}
	// The provider keeps this client for fetching its signing keys later on
	ctx := oidc.ClientContext(context.Background(), httpClient)

	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover the OIDC provider: %w", err)
	}

	return &oidcClient{
		config: config,
		oauth2: &oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			RedirectURL:  config.RedirectURI,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile"},
		},
		verifier:   provider.Verifier(&oidc.Config{ClientID: config.ClientID}),
		httpClient: httpClient,
	}, nil
}

// oidcIdentity is who signed in, as far as GADS is concerned
type oidcIdentity struct {
	Subject  string
	Username string
	Role     string
}

// identityFromClaims maps verified ID token claims to a GADS identity. The
// username is the provider's preferred_username only - falling back to the
// email would let an unverified address name an account
func identityFromClaims(subject string, claims map[string]any, config models.OIDCConfig) (oidcIdentity, error) {
	if subject == "" {
		return oidcIdentity{}, errors.New("the ID token has no subject")
	}

	username, _ := claims["preferred_username"].(string)
	if username == "" {
		return oidcIdentity{}, errors.New("the ID token has no preferred_username claim")
	}

	groupsClaim := config.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}

	role := "user"
	groups, _ := claims[groupsClaim].([]any)
	for _, group := range groups {
		if name, ok := group.(string); ok && config.AdminGroup != "" && name == config.AdminGroup {
			role = "admin"
			break
		}
	}

	return oidcIdentity{Subject: subject, Username: username, Role: role}, nil
}

// verifySignIn redeems the authorization code of a sign-in and checks the ID
// token it brings against the attempt that started it
func (o *oidcClient) verifySignIn(ctx context.Context, code string, login ssoLogin) (oidcIdentity, error) {
	if code == "" {
		return oidcIdentity{}, errors.New("no authorization code in the callback")
	}
	ctx = oidc.ClientContext(ctx, o.httpClient)

	token, err := o.oauth2.Exchange(ctx, code, oauth2.VerifierOption(login.verifier))
	if err != nil {
		return oidcIdentity{}, fmt.Errorf("failed to redeem the authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return oidcIdentity{}, errors.New("no ID token in the token response")
	}

	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return oidcIdentity{}, fmt.Errorf("failed to verify the ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(login.nonce)) != 1 {
		return oidcIdentity{}, errors.New("the ID token nonce does not match the sign-in")
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return oidcIdentity{}, fmt.Errorf("failed to read the ID token claims: %w", err)
	}

	return identityFromClaims(idToken.Subject, claims, o.config)
}
