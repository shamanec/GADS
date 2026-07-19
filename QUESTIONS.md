# GADS Fleet UI — Open Questions / API Gaps

Documented ambiguities encountered during Fleet UI build. Backend is frozen; these need product/operator decisions or future hub work.

## Selenium jar upload (Files tab)

- **UI:** "Upload Selenium jar" card with Select and upload button (matches stock GADS admin).
- **Backend:** No `/admin/files/selenium` route in current hub (API-MAP Gaps #1).
- **Current behavior:** UI renders the card; upload attempts `POST /admin/files/selenium` and shows the error message in a toast.

## Volume / Back hardware keys

- **UI:** Tile and detail control strips fire Back, Volume−, Volume+.
- **Backend:** No dedicated provider routes; Android typically uses Appium `press_keycode` (BACK=4, VOLUME_DOWN=25, VOLUME_UP=24).
- **Current behavior:** Client calls `POST /device/:udid/pressKey` with keycodes; toast on failure. May require an active Appium session via `/grid`.

## Battery percentage on tiles

- **UI:** Battery icon + `%` on tiles (prototype shows values).
- **Backend:** `LocalHubDevice` / `DBDevice` do not expose battery (API-MAP Gaps #6).
- **Current behavior:** Shows battery icon with em dash (`—`) when online.

## Provider restart button

- **UI:** Restart icon on Providers table rows.
- **Backend:** No restart-provider HTTP route (API-MAP Gaps #4).
- **Current behavior:** Toast explains manual process restart required.

## Re-provision device

- **UI:** Re-provision icon on Devices admin table.
- **Backend:** No dedicated hub endpoint (API-MAP Gaps #5).
- **Current behavior:** Toast notes missing endpoint.

## Client credential Workspace field

- **UI:** Create form includes Workspace* select.
- **Backend:** `POST /client-credentials` accepts only `name` and `description`; tenant from JWT (API-MAP Gaps #3).
- **Current behavior:** Field shown for parity; value not sent on create.

## Register-as-Selenium-grid-node toggle

- **UI:** Toggle on device detail "Selenium grid & logs" card.
- **Backend:** Grid is hub `/grid`; no per-device register API (API-MAP Gaps #7).
- **Current behavior:** Toast on toggle; no API call.

## SSE auth for `/available-devices`

- **Note:** Router may list endpoint as unauthenticated; client sends `?token=` query on EventSource since Bearer headers are not supported on EventSource.

## Stream auth

- **Implementation:** MJPEG `<img src>` uses `?token=${jwt}` query param on stream URLs.

## Admin gate re-auth on tab navigation within admin

- **Behavior:** Password gate runs on every mount of `/admin/*` (leaving admin and returning re-prompts). Tab switches within an unlocked admin session do not re-prompt (matches `fleet.html`).

## Device detail Appium port

- **UI:** Shows Appium port field.
- **Backend:** Port not exposed in hub device list payload in API-MAP.
- **Current behavior:** Displays em dash until provider exposes it.

## Power key

- **UI:** Power button on detail hardware row.
- **Backend:** No documented power endpoint.
- **Current behavior:** Toast "not supported via API".
