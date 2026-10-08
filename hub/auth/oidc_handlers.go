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
	"GADS/common/api"
	"GADS/common/db"
	"GADS/common/models"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

//go:embed templates/sso_callback.html
var ssoTemplates embed.FS

var ssoCallbackPage = template.Must(template.ParseFS(ssoTemplates, "templates/sso_callback.html"))

const (
	ssoLoginPath    = "/auth/sso/login"
	ssoCallbackPath = "/auth/sso/callback"
	// ssoLoginTTL is how long a sign-in can take at the provider
	ssoLoginTTL = 10 * time.Minute
	// A sign-in attempt lives in a cookie named after the start of its state,
	// so sign-ins started in two tabs do not overwrite each other
	ssoCookiePrefix     = "gads_sso_"
	ssoCookieStateChars = 16
)

var (
	errLocalUserExists = errors.New("a local user with this username already exists")
	errSubjectMismatch = errors.New("the account belongs to another identity of the provider")
	errSSOUserChanged  = errors.New("the account was changed or deleted during the sign-in")
)

// ssoLogin is one sign-in attempt. It is kept in a cookie of the browser that
// started it, so only that browser can complete the callback - this is what
// the state protects - and the hub holds nothing between the two requests
type ssoLogin struct {
	state    string
	nonce    string
	verifier string
}

func newSSOLogin() (ssoLogin, error) {
	state, err := randomToken()
	if err != nil {
		return ssoLogin{}, err
	}
	nonce, err := randomToken()
	if err != nil {
		return ssoLogin{}, err
	}
	return ssoLogin{state: state, nonce: nonce, verifier: oauth2.GenerateVerifier()}, nil
}

func (l ssoLogin) cookieName() string {
	return ssoCookiePrefix + l.state[:ssoCookieStateChars]
}

func (l ssoLogin) cookieValue() string {
	return l.state + "." + l.nonce + "." + l.verifier
}

// ssoLoginFromRequest finds the sign-in attempt a callback with this state belongs to
func ssoLoginFromRequest(r *http.Request, state string) (ssoLogin, error) {
	if len(state) < ssoCookieStateChars {
		return ssoLogin{}, errors.New("invalid state")
	}

	cookie, err := r.Cookie(ssoCookiePrefix + state[:ssoCookieStateChars])
	if err != nil {
		return ssoLogin{}, errors.New("no sign-in with this state was started in this browser")
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(state)) != 1 {
		return ssoLogin{}, errors.New("the state does not match the sign-in started in this browser")
	}

	return ssoLogin{state: parts[0], nonce: parts[1], verifier: parts[2]}, nil
}

func setSSOLoginCookie(c *gin.Context, config models.OIDCConfig, name, value string, maxAge int) {
	path := ssoCallbackPath
	if redirectURL, err := url.Parse(config.RedirectURI); err == nil && redirectURL.Path != "" {
		path = redirectURL.Path
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(config.RedirectURI, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

// truncate shortens text taken from a request before it is logged
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SSOLoginHandler godoc
// @Summary      Start an SSO sign-in
// @Description  Redirects to the OpenID Connect provider to sign in
// @Tags         Hub - Authentication
// @Success      302
// @Failure      404  {object}  models.ErrorResponse
// @Failure      500  {object}  models.ErrorResponse
// @Router       /auth/sso/login [get]
func SSOLoginHandler(c *gin.Context) {
	client, err := currentOIDCClient()
	if err != nil {
		api.NotFound(c, "SSO is not configured")
		return
	}

	login, err := newSSOLogin()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to start an SSO sign-in - %s", err))
		api.InternalError(c, "Failed to start the SSO sign-in")
		return
	}

	setSSOLoginCookie(c, client.config, login.cookieName(), login.cookieValue(), int(ssoLoginTTL.Seconds()))
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, client.oauth2.AuthCodeURL(login.state, oidc.Nonce(login.nonce), oauth2.S256ChallengeOption(login.verifier)))
}

// SSOCallbackHandler godoc
// @Summary      Finish an SSO sign-in
// @Description  Callback of the OpenID Connect provider. Creates the user on the first sign-in and hands the session token to the UI
// @Tags         Hub - Authentication
// @Produce      html
// @Param        code   query  string  true  "Authorization code"
// @Param        state  query  string  true  "State of the sign-in"
// @Success      200    "Page that stores the session token and opens the UI"
// @Failure      400    {object}  models.ErrorResponse
// @Failure      401    {object}  models.ErrorResponse
// @Failure      403    {object}  models.ErrorResponse
// @Failure      404    {object}  models.ErrorResponse
// @Failure      500    {object}  models.ErrorResponse
// @Router       /auth/sso/callback [get]
func SSOCallbackHandler(c *gin.Context) {
	// The page carries a session token and the URL a one-time code - neither
	// should end up in a cache or in the Referer of the next request
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")

	client, err := currentOIDCClient()
	if err != nil {
		api.NotFound(c, "SSO is not configured")
		return
	}

	login, err := ssoLoginFromRequest(c.Request, c.Query("state"))
	if err != nil {
		slog.Warn(fmt.Sprintf("SSO callback refused - %s", err))
		api.BadRequest(c, "The SSO sign-in expired or was started in another browser, sign in again")
		return
	}
	// The attempt is over whatever happens next
	setSSOLoginCookie(c, client.config, login.cookieName(), "", -1)

	if providerError := c.Query("error"); providerError != "" {
		slog.Warn(fmt.Sprintf("SSO sign-in rejected by the provider - %q: %q", truncate(providerError, 64), truncate(c.Query("error_description"), 256)))
		api.Unauthorized(c, "The SSO provider did not sign you in")
		return
	}

	identity, err := client.verifySignIn(c.Request.Context(), c.Query("code"), login)
	if err != nil {
		slog.Warn(fmt.Sprintf("SSO sign-in failed - %s", err))
		api.Unauthorized(c, "SSO sign-in failed")
		return
	}

	err = provisionSSOUser(identity)
	switch {
	case errors.Is(err, errLocalUserExists):
		api.Forbidden(c, fmt.Sprintf("User `%s` signs in with a GADS password, not through SSO", identity.Username))
		return
	case errors.Is(err, errSubjectMismatch):
		slog.Warn(fmt.Sprintf("SSO sign-in as `%s` refused - %s", identity.Username, err))
		api.Forbidden(c, fmt.Sprintf("User `%s` belongs to another SSO identity", identity.Username))
		return
	case errors.Is(err, errSSOUserChanged):
		api.Conflict(c, "The account changed during the sign-in, sign in again")
		return
	case err != nil:
		slog.Error(fmt.Sprintf("Failed to provision SSO user `%s` - %s", identity.Username, err))
		api.InternalError(c, "Failed to sign in")
		return
	}

	scopes := []string{"user"}
	if identity.Role == "admin" {
		scopes = append(scopes, "admin")
	}

	defaultTenant, err := db.GlobalMongoStore.GetOrCreateDefaultTenant()
	if err != nil {
		api.InternalError(c, "Failed to get default tenant")
		return
	}

	token, err := GenerateJWT(identity.Username, identity.Role, defaultTenant, scopes, GetOriginFromRequest(c))
	if err != nil {
		api.InternalError(c, "Failed to generate token")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	err = ssoCallbackPage.Execute(c.Writer, struct{ Token, Username, Role string }{token, identity.Username, identity.Role})
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to render the SSO callback page - %s", err))
	}
}

// SSOStatusHandler godoc
// @Summary      SSO availability
// @Description  Reports whether users can sign in through the OpenID Connect provider
// @Tags         Hub - Authentication
// @Produce      json
// @Success      200  {object}  models.SSOStatusResponse
// @Router       /auth/sso/status [get]
func SSOStatusHandler(c *gin.Context) {
	api.OK(c, "", models.SSOStatus{Enabled: IsOIDCEnabled(), LoginURL: ssoLoginPath})
}

// provisionSSOUser creates the account of an SSO identity on its first sign-in
// and keeps its role in line with the provider afterwards
func provisionSSOUser(identity oidcIdentity) error {
	user, err := db.GlobalMongoStore.GetUser(identity.Username)
	if err == mongo.ErrNoDocuments {
		err = createSSOUser(identity)
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		// Another sign-in of the same identity created the account meanwhile
		user, err = db.GlobalMongoStore.GetUser(identity.Username)
	}
	if err != nil {
		return fmt.Errorf("failed to look up the user: %w", err)
	}

	changed, err := reconcileSSOUser(&user, identity)
	if err != nil || !changed {
		return err
	}

	matched, err := db.GlobalMongoStore.UpdateSSOUser(user.Username, user.OIDCSubject, user.Role)
	if err != nil {
		return err
	}
	if !matched {
		return errSSOUserChanged
	}
	return nil
}

func createSSOUser(identity oidcIdentity) error {
	defaultWorkspace, err := db.GlobalMongoStore.GetDefaultWorkspace()
	if err != nil {
		return fmt.Errorf("failed to get the default workspace: %w", err)
	}

	return db.GlobalMongoStore.InsertUser(models.User{
		Username:     identity.Username,
		Role:         identity.Role,
		WorkspaceIDs: []string{defaultWorkspace.ID},
		AuthSource:   models.AuthSourceOIDC,
		OIDCSubject:  identity.Subject,
	})
}

// reconcileSSOUser checks that an existing account belongs to the identity and
// brings it in line with what the provider says now. Only accounts created
// through SSO are touched - a local account with the same username is never
// taken over - and an SSO account stays bound to the identity that created it.
// It reports whether the account changed
func reconcileSSOUser(user *models.User, identity oidcIdentity) (bool, error) {
	if user.AuthSource != models.AuthSourceOIDC {
		return false, errLocalUserExists
	}
	if user.OIDCSubject != "" && user.OIDCSubject != identity.Subject {
		return false, errSubjectMismatch
	}

	changed := false
	if user.OIDCSubject == "" {
		user.OIDCSubject = identity.Subject
		changed = true
	}
	if user.Role != identity.Role {
		user.Role = identity.Role
		changed = true
	}
	return changed, nil
}
