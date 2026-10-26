<div align="center">

# Haepage

**A homelab homepage that finds your services, looks good out of the box, and stays tiny.**

Install it, open it, and your Docker services are already there as styled cards.
Share a theme, widget or layout with a single link.

![Status: pre-alpha](https://img.shields.io/badge/status-pre--alpha-orange)
![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue)
![Go](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)
![Preact](https://img.shields.io/badge/frontend-Preact-673AB8?logo=preact&logoColor=white)
![Platforms](https://img.shields.io/badge/linux-amd64%20%7C%20arm64%20%7C%20armv7-informational)

[Why Haepage](#why-haepage) · [Features](#features) · [Status](#status) · [How it works](#how-it-works) · [Configuration](#configuration) · [Themes](#themes) · [Roadmap](#roadmap) · [Contributing](#contributing)

</div>

> [!WARNING]
> **Haepage is in early development.** The plan, contracts and project layout are in place, but there is no usable release yet. The install commands below show what is planned. Watch or star the repo to follow along.

---

## Why Haepage

- **Zero-config first run.** Haepage reads Docker and listening ports, recognizes common apps, and gives them a name, an icon and a group. New services wait in an *Inbox* and never rearrange a layout you built.
- **Design without YAML.** A visual editor, a code panel, or plain JSON: all three edit the same `haepage.json`.
- **One-link sharing.** Themes, widgets, search engines and layouts are plain files. Paste a link to import one, just like sharing a custom CSS snippet.
- **Light on the server.** One static Go binary, no database, no runtime. The target is under 20 MB of RAM and near-zero CPU while idle.
- **Fast in the browser.** First-load JavaScript under 50 KB gzipped, server-rendered first paint, and an offline-capable PWA.
- **Rich themes that stay smooth.** Every theme has Full, Balanced and Lite effect tiers, and Haepage picks the right one for each device.

## Features

> Everything here is **planned**. The [Status](#status) table shows what exists today.

<table>
<tr><td width="50%" valign="top">

**Discovery and monitoring**
- Docker auto-discovery via the events stream (no polling)
- Known-apps list for names, icons and groups
- `haepage.*` Docker labels to override anything
- Listening-port discovery, with opt-in fingerprinting
- Health dots, response time, uptime history
- TLS and domain expiry, container update indicator
- `/metrics` endpoint, public status page

</td><td width="50%" valign="top">

**Design and editing**
- Android-style (Material You) and Apple-style (glass) themes
- Light and dark, with Full, Balanced and Lite effect tiers
- Drag-and-drop editor, code panel, live preview
- Wallpapers with pre-blurred glass (no expensive live blur)
- Custom CSS, `?theme=<url>` share links, theme lint
- Profiles (Home, Work, Guest) with time-based switching

</td></tr>
<tr><td valign="top">

**Search (for your browser homepage)**
- Instant local filter of services and bookmarks
- `/` to focus, arrows to move, Enter to open
- Bangs: `!g`, `!d`, `!yt`, `!gh`, plus your own
- Importable engine lists, Ctrl+K command palette

</td><td valign="top">

**Widgets**
- Clock, weather, system stats, bookmarks, notes, to-do
- RSS, calendar (ICS), release tracker
- **No-code API widget**: point it at any JSON API and pick fields
- Secrets stay on the server (`{{secrets.name}}`)
- Sandboxed custom widgets you can share by link

</td></tr>
<tr><td valign="top">

**Tailscale**
- Smart links pick the LAN, Tailscale or public URL based on how you opened the page
- Devices panel from your local `tailscaled`
- Login-free roles from `tailscale serve` identity headers

</td><td valign="top">

**Access and operations**
- Password login, roles, optional OIDC single sign-on
- Public and private views
- Admin-only container actions and log viewer (off by default)
- Alerts through push notification services and webhooks
- Backup and restore, bookmark import, Wake-on-LAN

</td></tr>
</table>

## Status

Estimates of where each part stands. The bars are updated as milestones land.

| | Part | Progress | What it means |
| :---: | --- | :---: | --- |
| ✅ | **Product and build plan** | <img src="docs/assets/progress/100.svg" width="80" alt="100%"> | Vision, two-mode architecture, data model, API contracts and milestones written down |
| 🚧 | **Foundations** | <img src="docs/assets/progress/90.svg" width="80" alt="90%"> | Server skeleton with `/api/ping`, frontend build with size budgets, CI, JSON Schemas. Today: 8 MB idle RAM, 0 % idle CPU, 6.6 MB binary, 5.8 KB of JS. |
| 🚧 | **Theme API v1** | <img src="docs/assets/progress/70.svg" width="80" alt="70%"> | [Contract written](docs/theme-api.md): variables, classes, light and dark, effect tiers. Both built-in themes follow it. Theme lint still to come. |
| ⬜ | **Static mode** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | The whole dashboard as plain files on any web server, no backend needed |
| ⬜ | **Server mode** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | One Go binary that serves the page, saves config and pushes live updates |
| ⬜ | **Docker auto-discovery** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Containers show up as cards on their own, with names and icons from the known-apps list |
| ⬜ | **Health checks** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Up and down dots with response time, checked on a jittered schedule |
| ⬜ | **Search** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Local filter, keyboard first, web fallback with bangs |
| ⬜ | **Android and Apple themes** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Two polished defaults, light and dark, each with three effect tiers |
| ⬜ | **Theme import by link** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Paste a CSS link and it applies, checked and cached by the server |
| ⬜ | **Widgets** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Built-in widgets plus the no-code API widget, with secrets kept server-side |
| ⬜ | **Visual editor** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Build a complete dashboard without opening a text editor |
| ⬜ | **Tailscale integration** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Smart links and the devices panel |
| ⬜ | **Login and public view** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Safe to expose on a domain, with guests seeing only what is marked public |
| ⬜ | **Packages** | <img src="docs/assets/progress/0.svg" width="80" alt="0%"> | Docker image, `.deb`, `.rpm`, plain binary and a one-line install script |

✅ done · 🚧 in progress · ⬜ not started

## How it works

Haepage is **one frontend that runs in two modes**. On load the page asks `/api/ping` what the backend can do and turns on only those features.

| | Static mode | Server mode |
| --- | :---: | :---: |
| Needs | Any web server | The `haepage` binary |
| Manual services, groups, layout, search | ✅ | ✅ |
| Themes, custom CSS, import by link | ✅ (browser fetches) | ✅ (server fetches, validates, caches) |
| Visual editor | ✅ saves in the browser, exports JSON | ✅ saves on the server |
| Smart links (LAN, Tailscale, public) | ✅ | ✅ plus the devices panel |
| Auto-detect services | — | ✅ Docker, ports, systemd |
| Health checks | Basic (browser reachability) | Full (HTTP, TCP, Docker state) |
| API widgets with secrets | — | ✅ via the server proxy |
| Login, roles, alerts, container actions | — | ✅ |

Both modes read the same `haepage.json`. To move from Static to Server mode, drop that file into the data folder.

```
 Docker events ─┐
 Listening ports ─┼─► match known apps ─► merge with your config ─► live updates (SSE) ─► browser
 systemd units ─┘                              ▲
                                     haepage.json (only changes when you do)
```

## Installation

> [!NOTE]
> None of these exist yet. They show the planned install paths, so you can tell us early if something does not fit your setup.

**Docker Compose** (planned). Docker access goes through a filtered, read-only socket proxy by default, so Haepage can never send write calls to Docker unless you allow it.

**One-line install** (planned) for Debian, Ubuntu and other systemd Linux on amd64, arm64 and armv7. It installs a static binary and a hardened systemd service.

**Static mode** (planned): unzip the static bundle onto any web server and edit `haepage.json`.

**Build from source** (works today, needs Go, Node.js 22+ and `make`; it serves a placeholder page for now):

```sh
git clone https://github.com/haelahpx/haepage
cd haepage
make build      # builds the frontend, then the Go binary with it embedded
./haepage       # http://localhost:8080
```

## Configuration

Everything you choose lives in one JSON file. Detected services are never written to it unless you pin, rename or accept them.

```jsonc
{
  "version": 1,
  "settings": { "title": "Home", "theme": "android", "fx": "auto", "search": "duckduckgo" },
  "groups": [{ "id": "media", "name": "Media", "order": 1 }],
  "services": [{
    "id": "media-server", "name": "Media Server", "group": "media", "icon": "media",
    "urls": {
      "lan": "http://192.168.1.10:8096",
      "tailscale": "http://server.example.ts.net:8096",
      "public": "https://media.example.com"
    },
    "check": { "type": "http", "expect": 200, "interval": 60 }
  }],
  "overrides": { "docker:media/media-server": { "name": "Movies", "pinned": true } }
}
```

See [`haepage.example.json`](haepage.example.json) for the full shape. A JSON Schema will be published with each release, so editors can autocomplete and validate the file.

**Docker labels** (planned) let you set things from your compose files:

```yaml
labels:
  haepage.name: Movies
  haepage.group: Media
  haepage.icon: media
  haepage.url.public: https://media.example.com
```

## Themes

A theme is a `theme.json` plus a `style.css`, or a single CSS file that sets a few variables:

```css
:root {
  --accent: #e5484d;
  --surface: #1a1a1f;
  --on-surface: #ededef;
  --radius: 14px;
}
:root[data-scheme="light"] { --surface: #ffffff; --on-surface: #1a1a1f; }
```

Host it anywhere and share `https://your-dashboard/?theme=https://example.com/my-theme.css`. The theme API is versioned (`"theme-api": 1`), so a theme you share today keeps working on later versions.

The two built-in themes live in [`themes/`](themes/).

## Roadmap

| Milestone | Goal |
| --- | --- |
| **M0 Foundations** | Go module, CI, size budgets, benchmark harness, theme API v1, known-apps list |
| **M1 Static mode** | Services, groups, search and the Android theme from a plain folder |
| **M2 Server mode** | Go binary with Docker discovery, health checks, live updates, packages |
| **M3 Themes** | Android and Apple themes at all tiers, import by link, wallpapers, PWA, Tailscale |
| **M4 Widgets** | Built-in widgets, no-code API widget, uptime history, metrics |
| **M5 Visual editor** | Drag and drop, pickers, profiles, bookmark import, backup |
| **M6 Power features** | Login, roles, OIDC, public view, command palette, alerts, container actions |
| **M7 Sharing** | Sandboxed custom widgets by link, community gallery, multi-server |
| **M8 Release** | `.deb`, `.rpm`, multi-arch image, docs site, security review |

## Project layout

```
cmd/haepage/      entry point
internal/         config, discovery, health, proxy, themes, tailscale, auth, notify
web/              Preact frontend, embedded into the binary
themes/           built-in Android and Apple themes
widgets/          built-in widgets
packaging/        Docker, systemd, deb, rpm
docs/user/        user documentation
```

## Contributing

The project is young, so this is a good moment to shape it. Issues about features, themes and your homelab setup are very welcome.

- Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat`, `fix`, `docs`, `style`, …). The [PR template](.github/pull_request_template.md) has the full list.
- Keep the public contracts (theme API, widget API, config schema) stable. Breaking changes need a new version.
- New dependencies need a short justification in the PR. Staying small is a feature.

## License

Haepage is licensed under the [Apache License 2.0](LICENSE).
