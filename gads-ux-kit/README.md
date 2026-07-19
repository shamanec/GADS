# GADS Fleet UI — starter kit

**Building with Cursor (recommended path):** open `CURSOR-MASTER-PROMPT.md`, follow its 3-line "How to use" header at the top, paste the rest into Cursor's agent chat. It is a literal, numbered, field-by-field execution plan — Cursor should not need to make design decisions, only follow it.

**Building with Claude Code instead:** see the "Claude Code path" section at the bottom of this file.

## Everything in this folder

| File / folder | What it is |
|---|---|
| `CURSOR-MASTER-PROMPT.md` | **Start here.** The full step-by-step build plan for Cursor. |
| `fleet.html` | **The UI spec.** Open it in a browser — a complete, clickable prototype of every screen: login, the device wall (live tiles, tap/scroll, drag-drop install, broadcast bar), the single-device view, and all 9 admin tabs. |
| `device-wall-mockup.html`, `device-detail-mockup.html` | Earlier static drafts, kept for reference only. `fleet.html` wins on any conflict. |
| `reference-screenshots/` (10 PNGs) | Real screenshots of the actual stock GADS v5.7.0 admin UI — ground truth for exact field names, labels, and helper text on every admin form. |
| `CLAUDE.md` | Project rules and boundaries (backend frozen, no copying `hub-ui`, design tokens). Doubles as `.cursorrules` content. |
| `agents/*.md` | `gads-api-cartographer` (how to map the backend API), `frontend-builder` (build-phase rules), `ux-reviewer` (the acceptance checklist your output is graded against). |
| `skills/*/SKILL.md` | Prose specs for the wall/tile/broadcast interaction model and for wiring auth/streams/control to the real backend. |
| `00-SEND-THIS-PROMPT.md` | The original Claude Code version of the kickoff prompt — only needed for the Claude Code path below. |

**All of the above is referenced explicitly inside `CURSOR-MASTER-PROMPT.md` section 0** — nothing here is meant to be read in isolation.

## What gets built
A new React frontend in `fleet-ui/` that **replaces every GADS screen**: login (GADS JWT auth), a wall of **12–30 phones live in a grid** where **each tile is drivable in place** — tap/scroll on the screen, Back/Home/Volume/Upload buttons, drag-and-drop install — a **3-dots → single-device view with every GADS setting**, **multi-select broadcast**, and a full 9-tab **Admin section** matching stock GADS field-for-field (Providers, Devices, Users, Files, Global settings, Workspaces, Secret keys, Client credentials, Custom actions). The Go backend stays completely frozen — auth, users, providers, streaming all keep running as-is; the new UI only calls its existing endpoints.

## Order of work (the kit enforces this)
**Read every file in section 0 of the master prompt → map the real API → scaffold → build screen-by-screen, verifying each against the real running hub before moving to the next → self-review against `agents/ux-reviewer.md`.**

## One watch-out
GADS's existing frontend (`hub-ui/`) is an empty git submodule pointing at a **separate, proprietary-licensed private repo**. Never try to clone or copy it — build the new app clean.

---

## Claude Code path
If you're using Claude Code instead of Cursor:
1. Fork/clone GADS: https://github.com/shamanec/GADS
2. Copy `CLAUDE.md` → repo root.
3. Copy `agents/*.md` → `.claude/agents/`.
4. Copy the `skills/*` folders → `.claude/skills/`.
5. Open the repo in Claude Code and paste the contents of `00-SEND-THIS-PROMPT.md` as your first message.
