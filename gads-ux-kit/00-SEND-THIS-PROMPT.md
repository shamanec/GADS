# Paste this as your FIRST message to Claude Code (inside the GADS fork repo)

---

**Mission.** I'm forking **GADS** (https://github.com/shamanec/GADS), a self-hosted mobile device farm. Its Go backend (hub + providers) already does everything I need — auth, device provisioning, live video streaming, remote control (tap / swipe / type / clipboard / app install / screenshot), a per-device Appium server, reservations, and workspaces. **I am NOT changing the backend.**

What's weak is the UI: you basically control **one device at a time**. I want a brand-new, beautiful frontend that shows a **whole wall of phones at once** — **12–30 live device tiles** in a responsive grid — where **each tile is drivable in place**:
- tap and **scroll directly on the live screen**, while the rest of the wall keeps streaming;
- a per-tile control strip: **Back · Home · Volume− · Volume+ · Upload**;
- **drag-and-drop a file onto a phone (or an upload button) to install** an `.apk`/`.ipa` to that device;
- a **3-dots menu on each tile → opens that one phone full-size with every GADS setting** (device info, install/uninstall apps, streaming protocol/FPS, clipboard, reserve, restart/reset, Appium session, Selenium grid, logs);
- **multi-select to broadcast** one action to several phones at once.

**Full parity — the new UI replaces ALL of GADS's screens, not just the wall:**
- **Login page (day 1, non-negotiable)** — GADS's own username/password auth; store the JWT and send it on every request + WebSocket connect. No screen is reachable without it.
- **Admin section** — Users (add/delete, roles, password reset), Providers (register/remove, status), Devices (add, assign to provider, remove), Workspaces (who sees which devices), and hub Settings (stream defaults, Appium grid link). Stock GADS also has Files, Global settings, Secret keys / OAuth2 client credentials, and Custom actions — the API map must cover them and they land as Admin tabs too.
- **Admin re-auth EVERY entry** — the fleet login is one password, but opening Admin always asks for the admin password again (verify via the GADS login endpoint; no caching, no "remember me"). See the gate modal in `fleet.html`.
- Admin can land as phase 2 if the wall ships first — until then the stock GADS admin page on the hub stays usable for setup chores. But it must land.

Every existing GADS capability must stay reachable.

**A clickable reference prototype is included: `fleet.html`** (plus `device-detail-mockup.html`). Open it in a browser — the real app must match its layout, interactions, and clean look (light SaaS style, no gamer aesthetics). It covers EVERY screen: login, the wall, in-tile tap/scroll + button feedback, drag-drop install, the broadcast bar, the single-device view, and the full Admin section (Users / Providers / Devices / Workspaces / Settings).

**Read `CLAUDE.md` first.** Then follow this order — no skipping:

1. **Map the backend.** Dispatch the `gads-api-cartographer` agent to read the Go source and produce `docs/API-MAP.md`: every REST route, WebSocket channel, and video-stream URL, with payload shapes. **Do not write any UI code until this file exists.**
2. **Show me the plan.** A one-screen layout of the device wall + which APIs power each piece. Wait for my OK.
3. **Build** using the `building-the-device-wall` skill, the `talking-to-gads-backend` skill, and the `frontend-builder` agent. Put the new app in `fleet-ui/`.
4. **Verify on real devices.** I'll give you my running hub URL + a login. Confirm real streams and real taps work. **Screenshot the wall before you call anything done.**

**Hard constraints:**
- Never edit anything under `hub/` or `provider/` Go code. Only consume the existing HTTP / WebSocket / stream endpoints. If the backend is missing something the wall needs, tell me — don't patch it.
- Do **not** copy or modify `hub/gads-ui` — it's under a separate proprietary license. Build a clean new app from scratch.
- Keep many live streams from melting the browser: thumbnails low-FPS/downscaled, full-res only for focused or selected devices.

Start by reading `CLAUDE.md` and the repo structure, then give me a one-screen plan. Ask me for the hub URL + credentials when you're ready to test.
