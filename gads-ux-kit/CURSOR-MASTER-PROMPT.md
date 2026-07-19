# CURSOR MASTER PROMPT — GADS Fleet UI

> **How to use:** clone https://github.com/shamanec/GADS, copy this entire `gads-ux-kit/` folder into the repo root, open the repo in Cursor, paste everything below the `===` line into Cursor's agent chat as one message. Do not summarize or skip sections — execute them in order.

This document is written so you make almost no design or architecture decisions yourself. Where a decision is normally yours to make, it's already made below. If you hit a real ambiguity not covered here, write it to `QUESTIONS.md` in the repo root and keep going — do not stop the build to ask.

===

## 0. Read every file in this kit before writing any code

This folder (`gads-ux-kit/`) is not a suggestion — it is the complete spec. Read every file below, in this order, before touching `fleet-ui/`.

| # | File | What it is | What you do with it |
|---|---|---|---|
| 1 | `CLAUDE.md` | Project rules and boundaries | Copy its content into `.cursorrules` in the repo root right now, before anything else. These rules apply to every step below. |
| 2 | `fleet.html` | **The UI spec.** A complete, working, single-file HTML/CSS/JS prototype of the ENTIRE app — every screen, click, form field, animation | Open it in a real browser (double-click it, or `python3 -m http.server` in this folder). Click through all of it: login, the wall, drag-and-drop, the broadcast bar, single-device view, and all 9 admin tabs. This is what you are rebuilding in React. Match its layout, spacing, copy, and behavior exactly — do not restyle it, do not "improve" it, do not invent new screens. |
| 3 | `device-detail-mockup.html` | Earlier static draft of the single-device view | Reference only if `fleet.html` is ambiguous on that screen. `fleet.html` wins on any conflict. |
| 4 | `device-wall-mockup.html` | Earlier static draft of the device wall | Same — reference only, `fleet.html` wins. |
| 5 | `reference-screenshots/*.png` (10 files) | Real screenshots of the ACTUAL stock GADS v5.7.0 admin UI, taken by the project owner | These are ground truth for exact field names, exact button labels, exact helper text, and exact tab order in the real backend's admin panel. Section 4 below already transcribes every field from these — but if anything in section 4 seems unclear, open the corresponding screenshot and read it directly. Filenames map to screens as follows: `20.26.28`=Providers (Add+Update form), `20.26.31`=Providers+Devices side by side, `20.26.34`=Users (Add+Update form), `20.26.36`=Files, `20.26.39`=Global Settings, `20.26.41`=Workspaces, `20.26.45`=Secret Keys, `20.26.51`=Devices page (top-nav, non-admin), `20.27.04`=Client Credentials, `20.27.06`=Custom Actions. |
| 6 | `skills/building-the-device-wall/SKILL.md` | Prose spec of the wall/tile/broadcast interaction model | Read for the "why" behind the wall's behavior — tile anatomy, direct-interaction rule, multi-select broadcast. Everything in it is also in `fleet.html`; this explains the reasoning. |
| 7 | `skills/talking-to-gads-backend/SKILL.md` | Notes on wiring auth, streams, and control calls to the real Go backend | Read before you write any API integration code (Phase 3 below). |
| 8 | `agents/gads-api-cartographer.md` | The exact method for mapping the backend's API surface | Follow this method literally for Phase 1 below — it is not optional background reading, it is your instructions for that phase. |
| 9 | `agents/frontend-builder.md` | Build-phase rules (performance, boundaries) | Already folded into section 2 and 5 below; read once for context. |
| 10 | `agents/ux-reviewer.md` | The acceptance checklist a reviewer will use on your output | This is what "done" is graded against — read it now so you build to pass it, not after. |

**If any file in this table is missing from the folder you were given, stop and say so before proceeding — do not substitute your own design.**

===

## 1. Non-negotiable decisions (do not re-decide these)

- **Stack**: React 18 + TypeScript + Vite. State/data: TanStack Query for REST, native `WebSocket`/`EventSource` for live updates. Styling: plain CSS with CSS custom properties (see tokens below) — no Tailwind, no MUI, no component library. Router: `react-router-dom`.
- **New app location**: `fleet-ui/` at the repo root, sibling to `hub/`, `provider/`, `common/`.
- **Never touch**: `hub/`, `provider/`, `common/`, `appium-plugin/`, any `.go` file, `main.go`. You consume the running hub's HTTP/WebSocket/SSE endpoints only.
- **Never copy from or reference `hub-ui/`.** It is an empty git submodule pointing at a private, separately-licensed repo. Do not attempt to clone or fetch it.
- **Design tokens** (from `fleet.html` — copy these CSS custom properties verbatim into `fleet-ui/src/theme.css`):
  ```css
  :root{
    --bg:#f6f7f8; --panel:#ffffff; --tx:#17181a; --tx2:#5c6066; --tx3:#9a9fa6;
    --bd:#e4e6e9; --bd2:#d0d3d8; --acc:#2563eb; --acc-soft:#eef3fe; --acc-bd:#bcd0f7;
    --ok:#16a34a; --warn:#d97706; --danger:#dc2626; --r:10px;
  }
  ```
  Sidebar is a fixed 56px icon rail (see `fleet.html` `.side` class). Cards use 1px `var(--bd)` borders and `12px` border-radius. No gradients, no box-shadow except the small elevation on modals/the sidebar-adjacent detail panel already in `fleet.html`. No emoji anywhere in the UI.
- **Dev proxy**: `fleet-ui/vite.config.ts` proxies `/api`-equivalent calls to the Go hub per `docs/hub.md` in the GADS repo (the hub already serves the built frontend from the same origin in production; in dev, proxy to `http://localhost:10000`).
- **Auth model**: one JWT for the whole app (`POST /authenticate`), stored in memory + `sessionStorage`, attached to every request and WebSocket/SSE connection as `Authorization: Bearer <token>`. **Admin section additionally re-prompts for the password every single time it's entered** (see Phase 4) — this is a second, always-fresh check, not a second token type. Verify the re-entered password by calling `POST /authenticate` again with the current username; do not just compare strings client-side.

===

## 2. Phase 1 — Map the real API (do this before any UI code)

Follow `agents/gads-api-cartographer.md` exactly. Concretely:

1. Read `hub/router/routes.go`, `hub/auth/*.go`, `common/db/*.go`, and `provider/` for the endpoints the hub exposes.
2. Produce `docs/API-MAP.md` listing, for every endpoint: method, path, auth requirement, request body shape, response body shape, and one real example. Cover at minimum:
   - `POST /authenticate`, `POST /logout`, `GET /user-info`, `POST /oauth/token`
   - `GET /available-devices` (SSE), `GET /admin/provider/:nickname/info` (SSE)
   - `GET|POST|PUT|PATCH|DELETE /device/:udid/*path` (device proxy — tap/swipe/type/keys/screenshot/app install go through here)
   - `GET|POST /provider/:name/*path` (provider proxy)
   - `POST /devices/control/:udid/lock`, `POST /devices/control/:udid/unlock`, `GET /devices/control/:udid/in-use` (WS), `GET /devices/control/:udid/adb-tunnel`
   - `GET /appium-logs`, `GET /health`
   - Every `/admin/*` CRUD route for providers, devices, users, workspaces, secret keys, files, global settings, client credentials, custom actions.
3. For each of the 9 admin tabs in section 4 below, write the exact endpoint(s) that power it next to the tab name inside `API-MAP.md`.
4. If a UI element in `fleet.html` or section 4 has no corresponding backend endpoint, add it to a "Gaps" section at the bottom of `API-MAP.md` instead of inventing one.

Do not write any `fleet-ui/` component code until `docs/API-MAP.md` exists and is committed.

===

## 3. Phase 2 — Scaffold

1. `npm create vite@latest fleet-ui -- --template react-ts`, install `@tanstack/react-query`, `react-router-dom`.
2. Set up the dev proxy (section 1).
3. Create `src/theme.css` with the tokens from section 1, import it in `main.tsx`.
4. Create an `api/` module: an `authFetch(path, opts)` wrapper that attaches the bearer token and redirects to `/login` on 401, plus typed functions for each endpoint from `API-MAP.md`.
5. Set up routes: `/login`, `/` (wall), `/device/:udid` (single-device view), `/admin/*` (admin, gated).

===

## 4. Phase 3 — Build every screen, in this order

Build and manually verify each screen against the real hub (Phase 5) before moving to the next. Do not batch all screens then test at the end.

### 4.1 Login (`/login`)
Centered card exactly as in `fleet.html` `#login`: app icon, "GADS Fleet" heading, "Sign in to your hub" subtext, Username input, Password input, "Sign in" button. On submit, `POST /authenticate`; store the token; redirect to `/`. Show an inline error on failure — do not silently fail.

### 4.2 Device wall (`/`)
Reproduce `fleet.html`'s `.wall`/`.grid`/`.tile` structure exactly:
- Top bar: app icon + "Devices" + `{count} devices · {online} online`, search input, filter chips (All / iOS / Android / Free / In use / Offline), a workspace `<select>`.
- Responsive grid, `auto-fit, minmax(158px, 1fr)`, gap `13px`.
- Each tile, top to bottom: header row (status dot, name — click opens `/device/:udid`, battery + icon, 3-dots button — click also opens `/device/:udid`); live screen area (real stream once wired, see below); control strip of 5 buttons (Back, Home, Volume−, Volume+, Upload); footer (OS icon, provider name, selection checkbox).
- Wire the device list to `GET /available-devices` (SSE) — do not poll.
- Wire each tile's screen to the real stream endpoint from `API-MAP.md` (MJPEG = plain `<img src>`; WebRTC = the negotiation flow documented there). Thumbnails render at reduced FPS/scale; do not stream full-res to every tile at once — this will be verified by opening 10+ tiles and checking CPU/network in devtools.
- Tap on a tile's screen → `POST` to the device-control tap endpoint with the click's coordinate mapped to device resolution. Scroll on a tile's screen → swipe/scroll call. Back/Home/Volume buttons → their respective control endpoints.
- Drag a file onto a tile's screen (or click its Upload button to open a file picker) → app-install call for that device's UDID.
- Checkbox selects a tile → a broadcast bar slides up from the bottom (`fleet.html` `.bcast`) showing "Acting on N devices" with Tap / Type / Home / Install app buttons that fire the same call to every selected device in parallel, showing a per-device success/fail toast — never fail silently on one device without saying which.
- Unmount/tear down any stream whose tile has scrolled out of the viewport (IntersectionObserver). Virtualize the grid once device count exceeds ~30 (e.g. `react-window`).

### 4.3 Single-device view (`/device/:udid`)
Reproduce `fleet.html`'s `.detail` panel: back-to-fleet button, device name + status pill, provider label, top-left large live stream with hardware key buttons (Back, Home, Recents, Volume−, Volume+, Power) and Screenshot/Rotate buttons, and on the right a stack of cards:
- **Device**: Model, UDID, Screen (width × height), Appium port, Workspace, "Appium configuration" copy button, Reserved-by-you toggle (calls lock/unlock).
- **Apps**: drag-drop zone + browse button (install), list of installed apps with an uninstall trash icon each.
- **Streaming**: protocol segmented control (WebRTC / MJPEG), quality segmented control.
- **Session & actions**: Clipboard, Appium session, Restart, Reset buttons.
- **Selenium grid & logs**: register-as-node toggle, "Open logs" link.
`Esc` key and the back button both return to `/`.

### 4.4 Admin gate
Clicking any of the sidebar's Providers / Admin / Settings icons opens a modal exactly like `fleet.html`'s `#pwgate`: "Admin access" heading, "Enter the admin password — asked every time" subtext, password input, Unlock button, Cancel button. On submit, call `POST /authenticate` with the current username and the entered password; on success open `/admin`, on failure show an inline error and keep the modal open. **This check fires every single time Admin is entered — including navigating away and back within the same session.** Do not cache a "was unlocked" flag.

### 4.5 Admin — 9 tabs
Tab bar order, exactly: Providers, Devices, Users, Files, Global settings, Workspaces, Secret keys, Client credentials, Custom actions.

Every "Add" and every "Update" form below is a real, working form — every field must be present, required fields marked `*`, and submission must call the real backend endpoint from `API-MAP.md` and refresh the table on success. **No screen in this section may render with fewer fields than listed here.** This is the exact bug the project owner flagged: an earlier draft had "Add a device, no settings, no nothing" — that must not happen. Cross-check every field below against `reference-screenshots/` before marking a tab done.

**Providers** — table of existing providers (Nickname, OS, Host:Port, what it provides, WDA bundle, device count, live/down status, row actions: Show logs / Restart / Update / Delete). Add/Update form fields, in this order: OS* (select: Windows / macOS / Linux), Nickname* (text), Host address* (text), Port* (number), Provide iOS?* (select: No/Yes), Provide Android?* (select: No/Yes), Provide Tizen?* (select: No/Yes), Provide WebOS?* (select: No/Yes), Setup Appium servers?* (select: No/Yes), WDA bundle ID (text, only meaningful if iOS=Yes), iOS supervision profile password (password field). Buttons: "Add provider" (new) or "Update provider" + "Show logs" + "Delete provider" (existing). Footer note: "All updates to existing provider config require provider instance restart."

**Devices** — table of existing devices. Add/Update form fields, in this order: Device OS* (select: iOS/Android/Tizen/WebOS), Device type* (select: Real device/Emulator-Simulator), UDID* (text), Name* (text), OS Version* (text), Screen width (number), Screen height (number), Device usage* (select: Enabled/Disabled), Provider* (select, populated from the Providers list), Video stream type* (select: "WebRTC - Broadcast Extension" / "WebRTC - FFMpeg" / "MJPEG"), Workspace* (select, populated from the Workspaces list). Buttons: "Add device" (new) or "Update device" + "Re-provision device" + "Delete device" (existing). Footer note: "All updates to existing devices require respective provider restart."

**Users** — Add form fields: Username* (text), Password* (password), User role* (select: User/Admin), Workspaces* (select, populated from Workspaces). Update form for an existing user: same fields but Username shown read-only/greyed, Password optional (blank = unchanged). **Special case: the row for the `admin` username has NO Workspaces field at all** (implicit all-workspace access) — omit that field only for that one row, exactly as in `reference-screenshots/Screenshot 2026-07-18 at 20.26.34.png`. Buttons: "Add user" / "Update user" + "Delete user" (delete hidden for `admin`).

**Files** — three upload cards, exactly: (1) "Upload Selenium jar" — body text "If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended.", "Select and upload" button, status text "No uploaded file." / "File exists."; (2) "Upload supervision profile" — body text "Upload the supervision profile if you are using supervised iOS devices.", same button/status pattern; (3) "Upload WebDriverAgent IPA" — body text "Upload signed WebDriverAgent IPA file", same button/status pattern.

**Global settings** — two side-by-side cards. Card 1 "Stream Settings": Target FPS (select: 5/10/15/30 FPS), JPEG Quality (select: 50/75/90), Scaling Factor Android (select: 25/50/75/100%), Scaling Factor iOS (select: 25/50/75/100%), "Save Settings" button. Card 2 "TURN Server Configuration": intro text "Required for WebRTC behind NAT/firewalls. Uses secure ephemeral credentials.", an info callout "Security: Auto-expiring credentials. Generate secret: `openssl rand -base64 32`", "Enable TURN Server" toggle, TURN Server field (helper text "TURN server hostname or IP address"), Port field default 3478 (helper text "TURN server port (default: 3478)"), Shared Secret password field (helper text "Secret for credential generation"), TTL seconds field default 3600 (helper text "Credential lifetime (default: 3600s/1h)"), "Save TURN Config" button, a note callout "Changes apply immediately to new WebRTC connections.", and a "Setup help: See TURN Deployment Guide" link.

**Workspaces** — search box "Search workspaces", "Add Workspace" button, table columns: Workspace Name, Description, Tenant (truncated id + copy icon), Type (Default/Custom), Devices (count), Actions ("Edit" — the Default workspace has no delete). Add/Edit form: Workspace name*, Description.

**Secret keys** — search box "Filter by origin", "View History" + "Add Secret Key" buttons, table columns: Origin, User Identifier Claim, Tenant Identifier Claim, Status (badge), Created At, Updated At, Actions ("Edit" + "Disable"/"Enable" toggle button). Add/Edit form: Origin*, User identifier claim* (default "username"), Tenant identifier claim.

**Client credentials** — search box placeholder "Filter by name, description or client ID", "View Capabilities Example" + "Create New Credential" buttons, empty-state text exactly "No client credentials found. Add your first credential using the button above." when empty. Create form: Name*, Description, Workspace* — on success show the generated client ID and secret ONCE with a copy button (secrets are not re-shown after leaving the screen). Table when non-empty: Name, Description, Client ID (+copy), Workspace, Created, Delete.

**Custom actions** — info callout: "Custom Actions allow you to create reusable automation tasks (tap, swipe, text input, etc.) that can be executed on devices. Actions can be executed from the device control panel. You can mark up to 5 actions as favorites for quick access - favorites appear as buttons in the device panel, while other actions are available via dropdown. Only admins can create, edit, and delete custom actions.", "New Action" button, empty-state text exactly "No actions available. Click \"New Action\" to create one." when empty. Table when non-empty: favorite star toggle (max 5 favorited, block a 6th with a toast), Name, Description, Type (Tap/Swipe/Text input/Key press), Value, Edit/Delete.

===

## 5. Phase 4 — Wire it to the real backend and verify (do not skip)

A real hub instance is available for testing: run `./GADS hub --host-address=localhost --port=10000` (binary from the GitHub releases page, or `go build -tags ui .` per the repo README) with MongoDB reachable at `localhost:27017`. Login `admin` / `password`. Two real iPhones already exist in that database (iPhone XR, iPhone 11 Black, provider `mac-provider`, WDA bundle `com.reemkolm.wda.runner`) — they'll show offline unless a provider process is also running, which is fine for testing the admin CRUD and login flow even without a live stream.

Before calling any phase done:
- [ ] Login stores/attaches the JWT correctly; a 401 anywhere redirects to `/login`, not a blank screen.
- [ ] The wall renders the real device list from `GET /available-devices`, and every filter/search/workspace-select actually narrows the real list.
- [ ] Tap/scroll/Back/Home/Volume on a tile fire real requests (check the Network tab — not just optimistic UI with no call).
- [ ] Drag-drop install fires a real request with the real file.
- [ ] Broadcast fires to every selected device in parallel and reports per-device pass/fail.
- [ ] Leaving Admin and re-entering prompts the password again, every time, with no cached bypass.
- [ ] Every one of the 9 admin tabs' Add/Update/Delete forms round-trips to the real backend and the change survives a page reload.
- [ ] Every field listed in section 4.5 is actually present — grep your own JSX against this document's field lists before marking a tab complete.
- [ ] No stream: 10+ tiles open simultaneously doesn't visibly stutter the page (devtools performance check).

Run `agents/ux-reviewer.md`'s checklist against your own build as a final self-review before reporting the work as finished. Report results as PASS/FAIL per item, not a general summary.
