# GADS API Map (Fleet UI)

Mapped from hub/provider Go source for the Fleet UI. Auth: `Authorization: Bearer <jwt>` unless noted. Envelope for most JSON APIs:

```json
{ "success": true, "message": "...", "result": <T> }
```

---

## Auth

### `POST /authenticate` — public
- **Body:** `{ "username": "admin", "password": "password" }`
- **Response `result`:** `{ "access_token": "<jwt>", "token_type": "Bearer", "expires_in": 3600, "username": "admin", "role": "admin" }`
- **Example:** `curl -s -X POST http://localhost:10000/authenticate -H 'Content-Type: application/json' -d '{"username":"admin","password":"password"}'`

### `POST /logout` — auth
- **Body:** none
- **Response:** `{ "success": true, "message": "success" }` (client discards JWT)

### `GET /user-info` — auth
- **Response `result`:** username, role, tenant, scopes from JWT claims

### `POST /oauth/token` — public (client credentials)
- **Body (form or JSON):** `grant_type=client_credentials`, `client_id`, `client_secret`
- **Response:** OAuth2 access token for Appium/grid use

---

## Live device list & provider status

### `GET /available-devices?workspaceId=<id>` — SSE (listed unauthenticated in router; send Bearer anyway)
- **Produces:** `text/event-stream`; each event is a JSON array of hub devices
- **Item shape (LocalHubDevice):**
  ```json
  {
    "info": { "udid": "...", "os": "ios", "name": "iPhone XR", "os_version": "17.5", "provider": "mac-provider", "usage": "enabled", "screen_width": "828", "screen_height": "1792", "device_type": "real", "workspace_id": "...", "stream_type": "mjpeg" },
    "host": "localhost:10001",
    "connected": true,
    "provider_state": "live",
    "available": true,
    "in_use": false,
    "in_use_by": "",
    "is_running_automation": false
  }
  ```
- Polls ~1s. Filter by `workspaceId` (required).

### `GET /admin/provider/:nickname/info` — SSE
- Events: full `Provider` JSON every ~1s

### `GET /workspaces` — auth
- User-visible workspaces (for wall workspace select)

### `GET /ice-config` — auth
- WebRTC ICE/TURN credentials for browser negotiation

---

## Device reservation / in-use

### `POST /devices/control/:udid/lock` — auth (or `?token=`)
- **Query:** `ttl_minutes` (default 10, max 360)
- **Response:** `{ "udid", "locked_by", "tenant", "expires_at_ms" }`
- **409** if locked by another (admins can take over)

### `POST /devices/control/:udid/unlock` — auth
- Releases lock; **409** if locked by another (non-admin)

### `GET /devices/control/:udid/in-use?token=<jwt>` — WebSocket
- Keeps UI lock alive; server pings every 5s; inactivity 30min closes with code 4001
- Client must respond to keep lock; on disconnect releases UI lock unless API lease/automation active

### `GET /devices/control/:udid/adb-tunnel` — auth
- Proxies Android adb tunnel for local debugging

### `POST /admin/device/:udid/release` — admin
- Force-release device in use

---

## Hub ↔ provider proxies

### `ANY /device/:udid/*path` — auth
Forwards to provider host as `/device/:udid/*path`. Blocks legacy `*/appium` (use `/grid`). Requires device available and not locked by another user.

### `ANY /provider/:name/*path` — auth
Forwards to provider `host:port` as `/*path`.

### `POST /provider-update` — public (providers)
Provider heartbeat with device connected/state sync.

---

## Device control & streams (via hub proxy)

Base: `http://hub/device/{udid}/...` → provider.

| Method | Path (after `/device/:udid`) | Body / notes |
|--------|------------------------------|--------------|
| GET | `/android-stream-mjpeg` | MJPEG `multipart/x-mixed-replace` — use as `<img src>` |
| GET | `/ios-stream-mjpeg` | MJPEG (WDA or GADS stream) |
| GET | `/android-stream` | WS MJPEG frames |
| GET | `/ios-stream` | WS stream |
| GET | `/android-webrtc` | WebRTC WS negotiate |
| GET | `/ios-webrtc` | WebRTC WS |
| GET | `/ios-webrtc-broadcast` | Broadcast-extension WebRTC |
| POST | `/update-stream-settings` | per-device FPS/quality/scale |
| POST | `/tap` | `{ "x": 100, "y": 200 }` (`ActionData`) |
| POST | `/swipe` | `{ "x","y","endX","endY" }` |
| POST | `/touchAndHold` | `{ "x","y","duration" }` |
| POST | `/typeText` | `{ "text": "hello" }` |
| POST | `/home` | no body |
| POST | `/recents` | no body |
| POST | `/lock` / `/unlock` | screen lock (device), not hub reservation |
| POST | `/screenshot` | returns image |
| GET | `/getClipboard` | |
| POST | `/rotation` | `{ "rotation": "..." }` |
| GET | `/apps` | installed apps list |
| POST | `/uploadAndInstallApp` | `multipart/form-data` file |
| POST | `/uninstallApp` | app id/bundle |
| POST | `/launchApp` / `/closeApp` / `/killApp` | |
| POST | `/reset` | reset device |
| POST | `/custom-action` | execute custom action |
| GET | `/info` / `/health` / `/files` | |

Also: `POST /provider/:name/uploadFile` — install via provider.

**Volume / Back:** no dedicated provider routes. Android Back/Volume typically via Appium `press_keycode` (KEYCODE_BACK=4, VOLUME_DOWN=25, VOLUME_UP=24) through `/grid` session or undocumented paths — see Gaps.

---

## System

### `GET /health` — auth
`{ "success": true, "message": "ok" }`

### `GET /appium-logs?collection=&logLimit=` — auth
Appium plugin logs from Mongo collection.

### `ANY /grid/*path`
Experimental Appium/Selenium grid (session creation by UDID/platform).

---

## Admin — Providers tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/providers` | list `Provider[]` |
| POST | `/admin/providers/add` | body `Provider` |
| POST | `/admin/providers/update` | body `Provider` |
| DELETE | `/admin/providers/:nickname` | |
| GET | `/admin/providers/logs?collection=&logLimit=` | |
| GET | `/admin/provider/:nickname/info` | SSE live status |

**Provider body:**
```json
{
  "os": "darwin",
  "nickname": "mac-provider",
  "host_address": "localhost",
  "port": 10001,
  "provide_ios": true,
  "provide_android": false,
  "provide_tizen": false,
  "provide_webos": false,
  "setup_appium_servers": false,
  "wda_bundle_id": "com.reemkolm.wda.runner",
  "supervision_password": ""
}
```
(`os` values in UI: Windows / macOS / Linux — map to backend `os` string.)

---

## Admin — Devices tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/devices` | `{ devices, providers[], device_stream_types[] }` |
| POST | `/admin/device` | create `DBDevice` |
| PUT | `/admin/device` | update by `udid` |
| DELETE | `/admin/device/:udid` | |

**DBDevice:**
```json
{
  "udid": "...",
  "os": "ios",
  "name": "iPhone XR",
  "os_version": "17.5",
  "provider": "mac-provider",
  "usage": "enabled",
  "screen_width": "828",
  "screen_height": "1792",
  "device_type": "real",
  "workspace_id": "...",
  "stream_type": "mjpeg"
}
```
Stream type options from API: MJPEG, WebRTC-FFMpeg, Android WebRTC, iOS Broadcast Extension (exact enum strings from `models.StreamType`).

---

## Admin — Users tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/users` | passwords stripped |
| POST | `/admin/user` | create |
| PUT | `/admin/user` | update (blank password = leave unchanged in store if sent empty — verify) |
| DELETE | `/admin/user/:nickname` | not for `admin` |

**User:** `{ "username", "password", "role": "admin"|"user", "workspace_ids": ["..."] }`  
Non-admin users require ≥1 workspace. `admin` user: no workspace field in UI.

---

## Admin — Files tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/files` | list GridFS files + status |
| POST | `/admin/files/webdriveragent` | multipart `.ipa` (+ optional `description`) |
| POST | `/admin/files/webdriveragent/sign` | sign+upload WDA |
| POST | `/admin/files/broadcast` | broadcast extension IPA |
| POST | `/admin/files/broadcast/sign` | sign+upload broadcast |
| POST | `/admin/files/supervision` | `.p12` supervision profile (replaces) |
| POST | `/admin/files/csr` | generate CSR |
| DELETE | `/admin/files/:id` | |

**No Selenium-jar upload endpoint in current hub** — see Gaps.

---

## Admin — Global settings tab

| Method | Path | Notes |
|--------|------|-------|
| GET/POST | `/admin/global-settings` | `StreamSettings`: `target_fps`, `jpeg_quality`, `scaling_factor_android`, `scaling_factor_ios` |
| GET/POST | `/admin/turn-config` | `TURNConfig`: `enabled`, `server`, `port`, `shared_secret`, `ttl` |
| GET/POST | `/admin/minio-config` | MinIO (extra vs prototype) |
| GET | `/admin/system-status` | setup hints |

---

## Admin — Workspaces tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/workspaces` | list with device counts |
| POST | `/admin/workspaces` | `{ name, description }` |
| PUT | `/admin/workspaces` | update |
| DELETE | `/admin/workspaces/:id` | Default workspace not deletable |

---

## Admin — Secret keys tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/admin/secret-keys` | list (no raw key material) |
| POST | `/admin/secret-keys` | `{ origin, key?, is_default?, user_identifier_claim, tenant_identifier_claim, justification? }` |
| PUT | `/admin/secret-keys/:id` | update |
| DELETE | `/admin/secret-keys/:id` | disable |
| GET | `/admin/secret-keys/history` | audit |
| GET | `/admin/secret-keys/history/:id` | by id |

---

## Admin — Client credentials tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/client-credentials` | list for current user |
| POST | `/client-credentials` | `{ name, description }` → returns `client_id` + `client_secret` **once** |
| GET | `/client-credentials/:id` | |
| PUT | `/client-credentials/:id` | |
| DELETE | `/client-credentials/:id` | revoke |

UI "Workspace*" field: create API does not accept `workspace_id` (tenant comes from JWT) — see Gaps.

---

## Admin — Custom actions tab

| Method | Path | Notes |
|--------|------|-------|
| GET | `/custom-actions` | list |
| POST | `/custom-actions` | `{ name, description, action_type, parameters }` |
| PUT | `/custom-actions/:id` | |
| DELETE | `/custom-actions/:id` | |
| GET | `/custom-actions/favorites` | |
| POST | `/custom-actions/favorites/:id` | max 5 favorites (enforce in UI) |
| DELETE | `/custom-actions/favorites/:id` | |

`action_type`: tap / swipe / text / key etc. Execute on device: `POST /device/:udid/custom-action`.

---

## Admin tab → endpoints (quick index)

| Tab | Endpoints |
|-----|-----------|
| Providers | `GET/POST /admin/providers*`, SSE `/admin/provider/:nickname/info`, logs |
| Devices | `GET /admin/devices`, `POST/PUT/DELETE /admin/device*` |
| Users | `GET /admin/users`, `POST/PUT/DELETE /admin/user*` |
| Files | `GET /admin/files`, `POST /admin/files/{webdriveragent,broadcast,supervision,csr}`, `DELETE /admin/files/:id` |
| Global settings | `/admin/global-settings`, `/admin/turn-config` (+ optional minio) |
| Workspaces | `/admin/workspaces` CRUD |
| Secret keys | `/admin/secret-keys` + history |
| Client credentials | `/client-credentials` CRUD |
| Custom actions | `/custom-actions` + favorites |

---

## Five most important endpoints for the multi-device grid

1. **`GET /available-devices?workspaceId=` (SSE)** — live list + status for every tile  
2. **`GET /device/:udid/android-stream-mjpeg` or `/ios-stream-mjpeg`** — thumbnail `<img src>`  
3. **`GET /device/:udid/ios-webrtc` / `android-webrtc`** — full-res focus control  
4. **`POST /device/:udid/tap` (and `/swipe`, `/home`, `/typeText`)** — in-tile interaction  
5. **`POST /devices/control/:udid/lock` + `GET .../in-use` WS** — reservation while controlling  

---

## Gaps (UI wants it; no inventing endpoints)

1. **Selenium jar upload** — stock UI Files card; current hub has WDA / broadcast / supervision / CSR only. No `/admin/files/selenium` route.  
2. **Dedicated Volume± / Back provider routes** — UI control strip needs them; use Appium keycodes via an active session or document as unsupported without Appium.  
3. **Client credential Workspace field** — UI form asks Workspace*; create API binds only name/description (tenant from JWT).  
4. **Provider restart button** — UI has Restart; hub has no restart-provider HTTP route (operator restarts process).  
5. **Re-provision device** — UI button; no dedicated hub endpoint found (provider-side only).  
6. **Battery % on tiles** — `LocalHubDevice` / `DBDevice` do not expose battery; UI can hide or show "—".  
7. **Register-as-Selenium-grid-node toggle** on device detail — grid is hub `/grid`; no per-device register toggle API found.
