/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/models"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// W3C WebDriver error codes and their HTTP statuses (https://www.w3.org/TR/webdriver2/#errors)
type W3CError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func w3cInvalidArgument(msg string) *W3CError {
	return &W3CError{HTTPStatus: 400, Code: "invalid argument", Message: msg}
}

func w3cInvalidSessionID(msg string) *W3CError {
	return &W3CError{HTTPStatus: 404, Code: "invalid session id", Message: msg}
}

func w3cSessionNotCreated(msg string) *W3CError {
	return &W3CError{HTTPStatus: 500, Code: "session not created", Message: msg}
}

func writeW3CError(c *gin.Context, e *W3CError) {
	c.JSON(e.HTTPStatus, createErrorResponse(e.Message, e.Code, ""))
}

// gridStore is the subset of db.GlobalMongoStore the grid handlers use,
// as a seam so handlers can be tested with a fake store
type gridStore interface {
	GetClientCredentialBySecret(clientSecret string) (models.ClientCredentials, error)
	GetUser(username string) (models.User, error)
	GetUserWorkspaces(username string) []models.Workspace
	GetWorkspaces() ([]models.Workspace, error)
	GetOrCreateDefaultTenant() (string, error)
}

var gridDB gridStore

// InitGridStore sets the store used by the Appium grid handlers, called once at hub startup
func InitGridStore(store gridStore) {
	gridDB = store
}

// gridCandidate is one merged capabilities candidate (alwaysMatch merged into a firstMatch entry,
// or the legacy desiredCapabilities object) with the fields the grid matches on extracted
type gridCandidate struct {
	Caps map[string]interface{}

	PlatformName    string
	AutomationName  string
	PlatformVersion string
	DeviceUDID      string
	// Seconds; nil when the capability is absent, explicit 0 means Appium's idle timer is disabled
	NewCommandTimeout *int64
	// Seconds; nil when absent, from `<prefix>:queueTimeout` (consumed by the session queue)
	QueueTimeout *int64
}

type GridSessionRequest struct {
	// The full session request body as sent by the client
	Raw map[string]interface{}
	// Merged candidates in W3C matching order
	Candidates []gridCandidate
}

// parseSessionRequest parses a new-session request body per the W3C capabilities
// processing rules (https://www.w3.org/TR/webdriver2/#processing-capabilities), with a
// legacy fallback to `desiredCapabilities` when no W3C `capabilities` object is present
func parseSessionRequest(body []byte, prefix string) (*GridSessionRequest, *W3CError) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, w3cInvalidArgument(fmt.Sprintf("Failed to parse the session request body as JSON - %s", err.Error()))
	}

	req := &GridSessionRequest{Raw: raw}

	capsValue, hasCaps := raw["capabilities"]
	if hasCaps {
		caps, ok := capsValue.(map[string]interface{})
		if !ok {
			return nil, w3cInvalidArgument("`capabilities` in the session request is not a JSON object")
		}

		alwaysMatch := map[string]interface{}{}
		if amValue, ok := caps["alwaysMatch"]; ok {
			am, ok := amValue.(map[string]interface{})
			if !ok {
				return nil, w3cInvalidArgument("`capabilities.alwaysMatch` is not a JSON object")
			}
			alwaysMatch = am
		}

		firstMatch := []interface{}{map[string]interface{}{}}
		if fmValue, ok := caps["firstMatch"]; ok {
			fm, ok := fmValue.([]interface{})
			if !ok {
				return nil, w3cInvalidArgument("`capabilities.firstMatch` is not a JSON list")
			}
			if len(fm) > 0 {
				firstMatch = fm
			}
		}

		for i, entryValue := range firstMatch {
			entry, ok := entryValue.(map[string]interface{})
			if !ok {
				return nil, w3cInvalidArgument(fmt.Sprintf("`capabilities.firstMatch[%d]` is not a JSON object", i))
			}

			merged := make(map[string]interface{}, len(alwaysMatch)+len(entry))
			for k, v := range alwaysMatch {
				merged[k] = v
			}
			for k, v := range entry {
				if _, exists := merged[k]; exists {
					return nil, w3cInvalidArgument(fmt.Sprintf("Capability `%s` is present in both alwaysMatch and firstMatch[%d]", k, i))
				}
				merged[k] = v
			}

			candidate, w3cErr := extractCandidate(merged, prefix)
			if w3cErr != nil {
				return nil, w3cErr
			}
			req.Candidates = append(req.Candidates, candidate)
		}

		return req, nil
	}

	// Legacy fallback - no W3C `capabilities` object at all, use `desiredCapabilities` as a single candidate
	if desiredValue, ok := raw["desiredCapabilities"]; ok {
		desired, ok := desiredValue.(map[string]interface{})
		if !ok {
			return nil, w3cInvalidArgument("`desiredCapabilities` in the session request is not a JSON object")
		}
		candidate, w3cErr := extractCandidate(desired, prefix)
		if w3cErr != nil {
			return nil, w3cErr
		}
		req.Candidates = append(req.Candidates, candidate)
		return req, nil
	}

	return nil, w3cInvalidArgument("The session request contains neither a `capabilities` nor a `desiredCapabilities` object")
}

func extractCandidate(caps map[string]interface{}, prefix string) (gridCandidate, *W3CError) {
	candidate := gridCandidate{
		Caps:            caps,
		PlatformName:    capString(caps, "platformName"),
		AutomationName:  capString(caps, "appium:automationName"),
		PlatformVersion: capString(caps, "appium:platformVersion"),
		DeviceUDID:      capString(caps, "appium:udid"),
	}

	newCommandTimeout, w3cErr := capSeconds(caps, "appium:newCommandTimeout")
	if w3cErr != nil {
		return candidate, w3cErr
	}
	candidate.NewCommandTimeout = newCommandTimeout

	queueTimeout, w3cErr := capSeconds(caps, fmt.Sprintf("%s:queueTimeout", prefix))
	if w3cErr != nil {
		return candidate, w3cErr
	}
	candidate.QueueTimeout = queueTimeout

	return candidate, nil
}

func capString(caps map[string]interface{}, key string) string {
	if value, ok := caps[key].(string); ok {
		return value
	}
	return ""
}

// capSeconds reads a duration-in-seconds capability that clients send either
// as a JSON number or as a numeric string (Appium tolerates both, so must we).
// Returns nil when the capability is absent
func capSeconds(caps map[string]interface{}, key string) (*int64, *W3CError) {
	value, ok := caps[key]
	if !ok {
		return nil, nil
	}

	switch v := value.(type) {
	case float64:
		seconds := int64(v)
		return &seconds, nil
	case string:
		seconds, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return nil, w3cInvalidArgument(fmt.Sprintf("Capability `%s` is not a valid number - `%s`", key, v))
		}
		return &seconds, nil
	default:
		return nil, w3cInvalidArgument(fmt.Sprintf("Capability `%s` must be a number or a numeric string", key))
	}
}

// candidateTargetOS derives the device OS a candidate targets - either platformName
// or appium:automationName suffices (requiring both is stricter than Appium itself)
func candidateTargetOS(candidate gridCandidate) string {
	if strings.EqualFold(candidate.PlatformName, "iOS") ||
		strings.EqualFold(candidate.AutomationName, "XCUITest") {
		return "ios"
	}

	if strings.EqualFold(candidate.PlatformName, "Android") ||
		strings.EqualFold(candidate.AutomationName, "UiAutomator2") {
		return "android"
	}

	if strings.EqualFold(candidate.PlatformName, "TizenTV") ||
		strings.EqualFold(candidate.AutomationName, "TizenTV") {
		return "tizen"
	}

	if strings.EqualFold(candidate.PlatformName, "lgtv") ||
		strings.EqualFold(candidate.AutomationName, "webos") {
		return "webos"
	}

	if strings.EqualFold(candidate.PlatformName, "Roku") ||
		strings.EqualFold(candidate.AutomationName, "roku") {
		return "roku"
	}

	return ""
}

// selectGridCandidate returns the first candidate, in firstMatch order, that either
// pins a device by UDID or targets a known device OS. A candidate that matches
// nothing is not an error per spec - the next one is tried
func selectGridCandidate(candidates []gridCandidate) (gridCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.DeviceUDID != "" || candidateTargetOS(candidate) != "" {
			return candidate, true
		}
	}
	return gridCandidate{}, false
}

// stripGadsCaps removes all `<prefix>:*` capabilities (client secret, grid directives)
// from every capabilities location in the session request before it is forwarded to
// Appium - they are grid-internal and the secret must never reach provider Appium logs
func stripGadsCaps(sessionReq map[string]interface{}, prefix string) {
	keyPrefix := prefix + ":"

	stripFromMap := func(value interface{}) {
		if m, ok := value.(map[string]interface{}); ok {
			for k := range m {
				if strings.HasPrefix(k, keyPrefix) {
					delete(m, k)
				}
			}
		}
	}

	if caps, ok := sessionReq["capabilities"].(map[string]interface{}); ok {
		stripFromMap(caps["alwaysMatch"])
		if firstMatch, ok := caps["firstMatch"].([]interface{}); ok {
			for _, entry := range firstMatch {
				stripFromMap(entry)
			}
		}
	}
	stripFromMap(sessionReq["desiredCapabilities"])
}
