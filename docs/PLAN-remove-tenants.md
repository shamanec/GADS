# Tenant Removal — Completed

## Context

The tenant system added multi-tenant isolation across JWT claims, device locking, custom actions, client credentials, workspaces, and UI components. This complexity was unnecessary and made the code harder to maintain with scattered tenant checks. It has been fully removed.

## Summary of Changes

**26 files changed, ~416 lines removed net.**

### Backend (Go)

| Area | Files Changed | What Was Done |
|------|--------------|---------------|
| **Models** | `common/models/models.go`, `custom_actions.go`, `responses.go` | Removed `Tenant` field from `ClientCredentials`, `Workspace`, `WorkspaceWithDeviceCount`, `CustomAction`, `UserFavoriteAction`, `SecretKeyResponse`, `SecretKeyRequest`, `UserInfoResponse`, `OAuth2TokenRequest`, `CreateCredentialResponse` |
| **DB layer** | `common/db/client_credentials.go`, `custom_actions.go`, `workspace.go` | Removed tenant from all queries, function signatures, and indexes. Deleted `common/db/tenant.go` entirely |
| **Auth/JWT** | `hub/auth/jwt.go`, `auth.go`, `secretstore.go` | Removed `Tenant` from `JWTClaims`, `GenerateJWT`, `ValidateJWT`, `SecretKey.TenantIdentifierClaim`, `AuthMiddleware`, `LoginHandler`, `GetUserInfoHandler` |
| **Client credentials** | `hub/auth/clientcredentials/clientcredentials.go` | Removed tenant from `CredentialStore` interface and all functions (`CreateCredential`, `GetCredential`, `ListCredentials`, `UpdateCredential`, `RevokeCredential`, `ValidateCredentials`) |
| **Router handlers** | `hub/router/oauth.go`, `client_credentials.go`, `custom_actions.go`, `workspace.go`, `secrets.go`, `proxy.go`, `routes.go`, `appiumgrid.go` | Removed tenant extraction from context, tenant parameters from all handler calls, tenant-based error handling, and tenant query params |
| **Device locking** | `hub/devices/hub_device.go` | Simplified from `(user, tenant)` keying to `user`-only. Removed `InUseByTenant` field. Updated `AcquireLock`, `IsLockedByOther`, `ReleaseLock` |
| **Hub startup** | `hub/hub.go` | Removed default tenant generation, workspace tenant assignment, and workspace tenant migration loop |
| **Tests** | `hub_device_test.go`, `jwt_test.go`, `clientcredentials_test.go`, `client_credentials_test.go` | Updated all test code to match new signatures. Removed tenant-specific test cases |

### Frontend (React)

| Area | Files Changed | What Was Done |
|------|--------------|---------------|
| **Auth context** | `contexts/Auth.js` | Removed `tenant` state, localStorage, and context value |
| **Device selection** | `DeviceSelection/DeviceBox.js` | Simplified lock check to username-only |
| **Admin panels** | `Workspaces/WorkspacesAdministration.js`, `Users/UsersAdministration.js`, `SecretKeys/SecretKeyForm.js`, `SecretKeys/SecretKeyList.js` | Removed tenant fields, columns, filters, and form inputs |
| **Client credentials** | `ClientCredentials/CredentialsSuccessDialog.js` | Removed tenant display section |
| **Frontend tests** | `WorkspacesAdministration.test.js`, `SecretKeyForm.test.js`, `SecretKeyList.test.js`, `index.test.js` | Removed tenant from mock data and assertions |

### Documentation

| File | What Was Done |
|------|---------------|
| `docs/client-credentials-management.md` | Removed tenant from credential info and copy fields |
| `docs/secret-keys.md` | Removed tenant identifier claim references |
| `docs/appium-credentials.md` | Changed "multi-tenant" to "multi-instance" |

## Database — No Cleanup Needed

Old `tenant` fields will remain in existing MongoDB documents (`workspaces`, `custom_actions`, `user_favorite_actions`, `client_credentials`, `global_settings`). This is harmless:

- **No code reads them** — the Go structs no longer have `Tenant` fields, so the BSON decoder silently ignores them
- **Old indexes** containing tenant still exist but cause no functional issues — they just take up some space. On next startup, `CreateUserFavoriteActionIndexes()` and `CreateClientCredentialIndexes()` will create the new indexes without tenant. The old ones will coexist; drop them manually if you want to reclaim space
- **The `global_settings` document** with `type: "default-tenant"` is inert — nothing reads it anymore

No migration scripts or manual database work is required.

## Verification

1. `go build ./...` — passes
2. `go test ./hub/auth/... ./hub/devices/... ./common/db/... ./hub/auth/clientcredentials/...` — all pass
3. Pre-existing test failures in `hub/router` (unrelated to tenant removal) remain unchanged
