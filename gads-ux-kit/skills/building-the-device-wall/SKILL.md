---
name: building-the-device-wall
description: Use when building or changing the GADS Fleet UI device grid, the in-tile live controls, drag-and-drop install, multi-select broadcast, or the single-device settings view — the spec for showing and driving many phones at once.
---

# Building the Device Wall

The product is a wall of **live, directly-drivable phones**. Every tile is interactive *in place* — you don't have to open a device to use it. "One device at a time" is exactly what we're replacing.

## Tile anatomy (the unit)
Top to bottom, a tile is:
1. **Header** — status dot · device name · battery · **3-dots menu** (opens the single-device view).
2. **Live screen** — the real device screen, streamed. **You tap and scroll directly on it** (pointer/touch events map to the device). Low-FPS/downscaled until interacted with or focused. The whole screen is also a **drag-and-drop target** — drop an `.apk`/`.ipa` and it installs to *that* phone.
3. **Hardware control strip** — a compact icon row right under the screen: **Back · Home · Volume− · Volume+ · Upload**. (iOS hides Back/keeps Home equivalents per platform.) Upload opens a file picker as the button alternative to drag-drop.
4. **Footer** — OS icon · provider · selection checkbox (for broadcast).

Never bury the live screen — controls frame it, they don't cover it.

## The direct-interaction rule (the core of this redesign)
- **Tap**: click on the screen → tap at the mapped device coordinate.
- **Scroll**: scroll/drag on the screen → swipe/scroll the device. The user scrolls a phone *while the rest of the wall keeps streaming around it.*
- **Hardware keys**: Back / Home / Volume map to the device's real key events.
- **Type**: when a tile's screen has focus, keystrokes go to the device.
- **Install**: drag a file onto a tile, or hit its Upload button.
- All of this happens **without leaving the wall**.

## The grid
- Responsive auto-fit columns, min tile width ~158–180px; good on a laptop and a big monitor.
- Density toggle: compact ↔ comfortable.
- Pinned top bar: search by name; filter by platform / OS version / provider / status.
- Online count + fleet health always visible.
- Virtualize once tiles exceed ~30 so scrolling stays smooth.

## Single-device view (the 3-dots)
Clicking a tile's **3-dots → that one phone, full-size, with every setting GADS exposes** beside it. Left: the enlarged live phone + full hardware controls (Back, Home, Recents, Volume, Power) + Screenshot/Rotate. Right, in sections:
- **Device** — model, OS version, UDID, screen size, provider, Appium port, reserve/release.
- **Apps** — drag-drop / browse to install `.apk`/`.ipa`; installed-apps list with uninstall.
- **Streaming** — protocol toggle (WebRTC / MJPEG), quality / FPS.
- **Session & actions** — clipboard get/set, start/stop Appium session, restart, reset.
- **Selenium grid & logs** — register-as-node toggle; open device/Appium logs.
A `Back to Fleet` control returns to the wall. This view is the home for everything too detailed for a tile — but nothing here should be the *only* way to do a common action (tap/scroll/keys/install stay on the tile).

## Login & Admin (full parity — not optional)
- **Login first**: centered card, username + password → GADS JWT. Nothing renders without a token; sign-out lives at the bottom of the sidebar.
- **Admin view** (sidebar icons → one view with tabs): **Users** — add/delete, role select (admin/user), password reset, workspace assignment · **Providers** — register/remove, live/down badge, restart, device count · **Devices** — add, assign to a provider (select), remove · **Workspaces** — create/delete, drives which devices a user sees · **Settings** — stream defaults (protocol, thumbnail FPS, focus quality), hub address/status, link to the Appium grid page.
- Every table row action gives visible feedback (toast/state change). Match `fleet.html`.

## Multi-select broadcast
- Select N tiles → a broadcast bar appears: *"Acting on 5 devices."*
- Mirror one action to all selected: tap, type, Home, launch/kill app, install the same file, reserve/release, screenshot-all.
- Show per-device ack/failure — a phone may be offline; never silently drop one.

## Feel
- Dark by default. Dense but calm. Transitions ease; nothing snaps.
- Color carries meaning (status), not decoration.

## Don't
- Don't make the user open a device just to tap or scroll — that's the old GADS flow we're killing.
- Don't run 30 full-res streams at once — thumbnails cheap, full streams expensive; upgrade a tile's stream when it's interacted with / focused.
- Don't hide a GADS capability to look clean — parity is non-negotiable.
