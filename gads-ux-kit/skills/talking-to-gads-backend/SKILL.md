---
name: talking-to-gads-backend
description: Use when wiring the Fleet UI to the GADS backend — auth tokens, REST calls, WebSocket device updates, and video streams (MJPEG / WebDriverAgent / WebRTC). Read docs/API-MAP.md first.
---

# Talking to the GADS Backend

The backend is GADS, untouched. Everything here consumes `docs/API-MAP.md` — if a detail isn't in there, run the `gads-api-cartographer` agent; don't guess.

## Auth
- Log in → get a JWT. Store it (memory + the refresh strategy from the API map).
- Send it on every REST request **and** when opening a WebSocket. A 401 means re-auth, not retry-forever.

## Live device state
- Device list / status (online, reserved, battery) comes over **WebSocket**, not polling. Subscribe once, fan updates out to tiles. Reconnect with backoff on drop.

## Video streams
- **Android**: MJPEG. A thumbnail is just `<img src={streamUrl}>` — cheap. For low FPS, the provider may expose a frame-rate or scale param; otherwise throttle by mounting/unmounting.
- **iOS**: via WebDriverAgent stream.
- **Low-latency full control**: WebRTC if the provider exposes it (experimental in GADS) — fall back to MJPEG.
- **Rule**: mount a full-res stream only for focused/selected tiles. Unmount streams for tiles scrolled out of view, or the browser dies.

## Remote control
- Tap / swipe / type / clipboard / app-install / reserve are REST (or WS) calls to the provider, usually **proxied through the hub** and addressed by device id. Each has a payload — see the API map.
- For **broadcast**, fire the same action across selected device ids in parallel and collect per-device results.

## Hub ↔ provider
- The hub proxies to the provider that owns a device. Address devices by id and let the hub route. **Don't hardcode provider hosts in the UI.**

## Dev setup
- GADS expects a dev proxy to the Go backend (see GADS `docs/hub.md`). Set that proxy so the new app can call the API with hot reload.
