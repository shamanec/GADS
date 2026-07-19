# GADS Fleet UI — project brief

A new frontend for a forked **GADS** device farm. The point: drive a **wall of phones at once**, beautifully — not one device at a time.

## What GADS already is (don't rebuild this)
- **Hub** (Go) — web server + auth (login, roles, JWT, OAuth2), workspaces, provider/device admin, proxy to providers, MongoDB. Serves REST + WebSocket.
- **Provider(s)** (Go) — provision devices, run a per-device Appium server, stream video (Android = MJPEG, iOS = WebDriverAgent, experimental WebRTC), and execute remote-control actions.
- **Capabilities to preserve**: live video per device, tap / swipe / text / clipboard / keyboard / screenshot, app install/uninstall, device reservation, Appium endpoints, Smart-TV (Tizen/WebOS), Selenium Grid.

## Our job (the only thing we build)
A standalone React frontend in `fleet-ui/` that consumes the existing backend APIs and delivers a multi-device UX.

### North star: the Device Wall
- Responsive grid of **12–30 device tiles**, each a **live screen** + status.
- **Every tile is interactive in place** — you tap and scroll directly on its screen, and each tile has a control strip: **Back · Home · Volume− · Volume+ · Upload**. No need to open a device just to use it. This is the core of the redesign.
- **Drag-and-drop install**: drop an `.apk`/`.ipa` onto a phone (or hit its Upload button) → installs to that device.
- Tile shows: model, OS + version, online/offline, free / reserved / in-use, battery, provider, a **3-dots menu**.
- **Filter / search / sort**: platform (iOS / Android / TV), OS version, provider, status.
- **3-dots → single-device view**: that one phone full-size with **every GADS setting** beside it (device info, app install/uninstall, streaming protocol/FPS, clipboard, reserve, restart/reset, Appium session, Selenium grid node, logs). `Back to Fleet` returns.
- **Multi-select → broadcast**: select N phones, mirror one action (tap / type / Home / install same file / reserve) to all at once.
- Color-coded fleet health, online count, dense and smooth.

### Full parity: login + admin are part of the job
- **Login (day 1)**: GADS's own username/password → JWT. Store it, send it on every request and WebSocket connect. Sign-out in the sidebar.
- **Admin section** (sidebar → tabs): **Users** (add/delete, admin/user role, password reset, workspace assignment) · **Providers** (register/remove, live/down status, restart) · **Devices** (add, assign to provider, remove) · **Workspaces** (access control) · **Settings** (stream defaults, hub info, Appium grid link).
- **Admin re-auth every entry**: opening the Admin area always prompts for the admin's password again — verify it against the GADS login endpoint; never cache or "remember" this.
- Stock GADS v5.7 also has: **Files** (uploaded files), **Global settings**, **Secret keys** + **Client credentials** (OAuth2), **Custom actions**. The API map (step 1) must cover these too — port them into Admin as simple tabs; don't drop them.
- The reference prototype `fleet.html` shows all of these working — match it.

## Golden rules
1. **Backend is frozen.** Never edit `hub/` or `provider/` Go. Consume its HTTP/WS/stream endpoints only. Missing something? Tell the human — don't patch the backend.
2. **Don't touch `hub/gads-ui`.** Separate *proprietary* license. Build a clean new app. Reference its behavior, never its code.
3. **API map before UI.** Nothing gets built until `docs/API-MAP.md` exists (the `gads-api-cartographer` agent makes it).
4. **Performance is a feature.** Many streams will kill a browser. Thumbnails = low-FPS / downscaled; full-res only for focused or selected devices; tear down streams when a tile leaves the viewport; virtualize the grid past ~30 tiles.
5. **Verify on real devices.** Test against a running hub — real streams, real taps — never assume. Screenshot before claiming done.
6. **Keep every capability reachable.** A redesign that drops "install app" or "reserve" is a regression, not a cleanup.

## Workflow
- Plan → confirm with the human → build → verify. Don't batch-build blind.
- `gads-api-cartographer` agent → learn the backend.
- `building-the-device-wall` skill → the UI spec/patterns.
- `talking-to-gads-backend` skill → streaming + control integration.
- `ux-reviewer` agent → review before merging.

## Suggested stack (adjust to what you find)
- React + TypeScript + Vite.
- TanStack Query for REST; native WebSocket hooks for live device state.
- Tailwind + a light component layer (shadcn/ui or MUI — your call).
- Virtualized grid (react-virtuoso / react-window) once you scale past ~30 tiles.

## Tech facts that matter
- **Streams**: Android MJPEG — a thumbnail is just `<img src={streamUrl}>`, which is cheap. iOS via WebDriverAgent. Experimental WebRTC for low-latency full control — use it for focus mode if available, fall back to MJPEG.
- **Auth**: JWT — store the token, send it on every request + on WebSocket connect.
- **Dev**: the Go hub serves the UI build; in dev, GADS expects a proxy to the backend (see GADS `docs/hub.md`). Set that up for hot reload.
