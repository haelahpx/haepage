# Widget API v1

This is the public contract for Haepage widgets: the config entry, each built-in type's `options`, the server endpoints widgets use, and the manifest for custom widgets. Anything built against `"widget-api": 1` keeps working on every release that supports version 1. Breaking changes only ever come with a new version number.

For a how-to, see the [widgets guide](user/widgets.md).

## Config entry

Each widget is one entry in `haepage.json` → `widgets[]` (schema: [`schema/haepage.schema.json`](../schema/haepage.schema.json)):

```json
{ "id": "stats", "type": "api", "title": "My App", "area": "header", "refresh": 60,
  "options": { "url": "http://192.168.1.2/api/stats", "display": "stat",
               "fields": [{ "label": "Users", "path": "users.total" }] } }
```

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | yes | `^[a-z0-9-]+$`, unique among widgets. Server endpoints address widgets by it. |
| `type` | yes | A built-in type below, or `custom:<id>` (custom widgets, planned) |
| `title` | no | Tile heading. Missing: the type's name. `""`: no heading. |
| `area` | no | `header` (default): with the top tiles. A group `id`: inside that group. `sidebar`, or a group that no longer exists: after the groups. |
| `refresh` | no | Seconds between updates, ≥ 1. Also the server's cache time for that widget (minimum 5 s). |
| `options` | no | Type-specific, below. Unknown keys are kept and ignored. |

A widget with an unknown `type` shows a short notice instead of breaking the page. A widget that throws shows **Try again**; other widgets are not affected.

## Built-in types

| `type` | `options` | Default `refresh` | Data from |
| --- | --- | --- | --- |
| `clock` | `timezone` (IANA name, `""` = the device's), `label` | — | the browser |
| `weather` | `latitude`, `longitude`, `name`, `units` (`metric` \| `imperial`) | 900 | a free public weather service, from the browser |
| `system` | — | page's stats poll | `GET /api/system` |
| `temperature` | — | page's stats poll | `GET /api/system` |
| `disks` | — | page's stats poll | `GET /api/system` |
| `bookmarks` | `links`: `[{ "name", "url" }]` | — | the config |
| `notes` | `text` | — | the config |
| `todo` | `items`: `[{ "id", "text", "done" }]` | — | the config |
| `rss` | `url`, `limit` (8) | 900 | server |
| `calendar` | `url` (`https://` or `webcal://`), `days` (14, max 366), `limit` (10) | 900 | server |
| `releases` | `repos`: `["owner/name"]` (≤ 20), `limit` (6) | 3600 | server |
| `expiry` | `domains`: `["example.com"]` (≤ 20), `warnDays` (30) | 3600 | server (domains); service health checks (certificates) |
| `api` | see below | 60 | server |

`system`, `temperature` and `disks` need the `system` capability. Types whose data comes from the server need `widgets.proxy`. Capabilities are listed by `GET /api/ping`; a widget whose capability is missing explains that instead of loading.

### `api` options

| Key | Meaning |
| --- | --- |
| `url` | Address of a JSON API. May contain `{{secrets.name}}`. |
| `headers` | `{ "Header-Name": "value" }`. Values may contain `{{secrets.name}}`. |
| `display` | `stat` (default), `progress`, `list` or `table` |
| `items` | `list`/`table`: path to the array in the answer (`""` = the answer itself) |
| `fields` | `[{ "label", "path", "format", "max" }]`. In `list`/`table`, paths are relative to each item. |

**Paths** use dots and brackets: `a.b[0].c`. A missing path shows `—`.

**Formats**: `auto` (default), `number`, `percent`, `bytes`, `duration` (seconds), `date`, `text`.

**`max`** (`progress` only): a number, or a path to the maximum in the answer.

## Server endpoints

### `GET /api/widgets/{id}/data`

Fetches the data for the widget `{id}` from the saved config, so a browser can only ask for addresses that are in the config. Used by `rss`, `calendar`, `releases`, `expiry` and `api`.

- `{{secrets.name}}` is filled in on the server. Secret values, and their URL-escaped forms, are replaced with `[secret]` in answers and error messages.
- Requests go through the server's address guard: local and private addresses are refused unless their host is in `settings.proxyAllow` or belongs to a service in the config. Redirects from `https` to `http` are refused.
- Upstream answers: at most 2 MB, 10 s timeout. `api` answers must be JSON.
- Answers are cached per widget for `refresh` seconds (minimum 5); a change to the widget's settings starts afresh.
- Rate limit: 60 requests per minute per client address.

| Status | Body |
| --- | --- |
| 200 | The answer (below), `Cache-Control: no-store` |
| 400 | `{ "error" }`: bad settings, an address the guard refused, a missing secret, an upstream error status, an answer that isn't the expected kind |
| 404 | `{ "error" }`: no widget with that id, or its type doesn't use this endpoint |
| 429 | Rate limit |
| 502 | `{ "error" }`: the upstream couldn't be reached or timed out |

Answer shapes:

| `type` | Answer |
| --- | --- |
| `api` | The upstream JSON as is |
| `rss` | `{ "title", "items": [{ "title", "link"?, "date"?, "summary"? }] }`, newest first, plain text |
| `calendar` | `{ "events": [{ "title", "location"?, "start", "end", "allDay"?, "date"? }] }`, in order, repeating events expanded |
| `releases` | `{ "items": [{ "repo", "title", "link"?, "date"?, "summary"? }], "errors": [{ "repo", "error" }] }`, newest first; a failing repository doesn't hide the others |
| `expiry` | `{ "domains": [{ "name", "expires"?, "registrar"?, "error"? }] }` |

Dates are RFC 3339.

### Secrets

| Route | Meaning |
| --- | --- |
| `GET /api/secrets` | `{ "names": [...] }`. Names only, never values. |
| `PUT /api/secrets/{name}` | Body `{ "value": "…" }`. An empty value deletes it. `204`. |
| `DELETE /api/secrets/{name}` | Deletes it. `204`. |

Names are `[a-z0-9_-]+`. Secrets are stored in `secrets.json` in the data folder. Keys that Haepage stores for its own integrations are not listed and can't be used in widgets.

## Custom widget manifest v1 (planned)

Custom widgets are not available yet. Their manifest is fixed now so shared widgets keep working ([`schema/widget.schema.json`](../schema/widget.schema.json)):

```json
{ "name": "Example", "id": "example", "version": "1.0.0", "widget-api": 1,
  "entry": "index.html",
  "permissions": { "network": ["https://api.example.com"], "secrets": ["example_token"], "storage": true },
  "size": { "w": 2, "h": 1 } }
```

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Display name |
| `id` | yes | `^[a-z0-9-]+$`; used as `custom:<id>` in the config |
| `version` | yes | The widget's own version |
| `widget-api` | yes | `1` |
| `entry` | yes | The HTML file to load |
| `permissions.network` | no | Origins the widget may fetch, always through the Haepage server |
| `permissions.secrets` | no | Secret names it may use (filled in on the server, never shown to it) |
| `permissions.storage` | no | Whether it may keep its own small storage |
| `size` | no | Preferred tile size, 1–4 by 1–4 |

A custom widget will run in a sandboxed frame without access to the page, and reach the outside only through messages to Haepage. You'll see and approve its permissions when you add it.
