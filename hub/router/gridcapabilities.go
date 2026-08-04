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
// processing rules (https://www.w3.org/TR/webdriver2/#processing-capabilities).
// A W3C `capabilities` object is required - legacy JSONWP `desiredCapabilities`-only
// bodies are rejected, same as Appium >= 2 itself does
func parseSessionRequest(body []byte, prefix string) (*GridSessionRequest, *W3CError) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, w3cInvalidArgument(fmt.Sprintf("Failed to parse the session request body as JSON - %s", err.Error()))
	}

	req := &GridSessionRequest{Raw: raw}

	capsValue, hasCaps := raw["capabilities"]
	if !hasCaps {
		return nil, w3cInvalidArgument("The session request contains no `capabilities` object - W3C capabilities are required, legacy `desiredCapabilities`-only bodies are not supported")
	}
	caps, ok := capsValue.(map[string]interface{})
	if !ok {
		return nil, w3cInvalidArgument("`capabilities` in the session request is not a JSON object")
	}

	// alwaysMatch is optional - default to an empty capabilities object
	alwaysMatch := map[string]interface{}{}
	if rawAlwaysMatch, present := caps["alwaysMatch"]; present {
		alwaysMatchCaps, ok := rawAlwaysMatch.(map[string]interface{})
		if !ok {
			return nil, w3cInvalidArgument("`capabilities.alwaysMatch` is not a JSON object")
		}
		alwaysMatch = alwaysMatchCaps
	}

	// firstMatch absent -> treat as a single empty entry `[{}]` (spec), so alwaysMatch
	// alone still produces one candidate
	firstMatch := []interface{}{map[string]interface{}{}}
	if rawFirstMatch, present := caps["firstMatch"]; present {
		firstMatchEntries, ok := rawFirstMatch.([]interface{})
		if !ok {
			return nil, w3cInvalidArgument("`capabilities.firstMatch` is not a JSON list")
		}
		// Present-but-empty is an error per spec - Appium rejects the body the same way
		if len(firstMatchEntries) == 0 {
			return nil, w3cInvalidArgument("`capabilities.firstMatch` must contain at least one entry when present")
		}
		firstMatch = firstMatchEntries
	}

	// Build one merged candidate per firstMatch entry (spec `#dfn-merging-capabilities`):
	// start from the alwaysMatch capabilities, then add the entry's own. The same
	// capability appearing in both places is an error per spec, not an overwrite
	for entryIndex, rawEntry := range firstMatch {
		firstMatchCaps, ok := rawEntry.(map[string]interface{})
		if !ok {
			return nil, w3cInvalidArgument(fmt.Sprintf("`capabilities.firstMatch[%d]` is not a JSON object", entryIndex))
		}

		mergedCaps := make(map[string]interface{}, len(alwaysMatch)+len(firstMatchCaps))
		for capName, capValue := range alwaysMatch {
			mergedCaps[capName] = capValue
		}
		for capName, capValue := range firstMatchCaps {
			if _, alsoInAlwaysMatch := mergedCaps[capName]; alsoInAlwaysMatch {
				return nil, w3cInvalidArgument(fmt.Sprintf("Capability `%s` is present in both alwaysMatch and firstMatch[%d]", capName, entryIndex))
			}
			mergedCaps[capName] = capValue
		}

		candidate, w3cErr := extractCandidate(mergedCaps, prefix)
		if w3cErr != nil {
			return nil, w3cErr
		}
		req.Candidates = append(req.Candidates, candidate)
	}

	return req, nil
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
	gadsKeyPrefix := prefix + ":"

	// Deletes every `<prefix>:*` key if the value is a capabilities object, no-op otherwise
	stripFromCapsObject := func(rawCapsObject interface{}) {
		if capsObject, ok := rawCapsObject.(map[string]interface{}); ok {
			for capName := range capsObject {
				if strings.HasPrefix(capName, gadsKeyPrefix) {
					delete(capsObject, capName)
				}
			}
		}
	}

	if caps, ok := sessionReq["capabilities"].(map[string]interface{}); ok {
		stripFromCapsObject(caps["alwaysMatch"])
		if firstMatchEntries, ok := caps["firstMatch"].([]interface{}); ok {
			for _, firstMatchEntry := range firstMatchEntries {
				stripFromCapsObject(firstMatchEntry)
			}
		}
	}
	// Old java-clients send a legacy `desiredCapabilities` object alongside the W3C one -
	// the secret must be stripped from there too before the body is forwarded
	stripFromCapsObject(sessionReq["desiredCapabilities"])
}
