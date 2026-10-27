# Widgets

Widgets are small tiles on your page: a clock, the weather, your server's stats, a to-do list, news, your calendar, or numbers from any app's API.

## Add, move and remove

Open **Settings → Widgets**:

- **Add** picks a widget type. Most widgets then ask for what they need right on the tile (a place for the weather, a feed address, a list of domains).
- **Place** chooses where it goes: **Top** (with the clock and search tiles), inside one of your **groups**, or **Bottom** (after the groups).
- **Remove** takes it off the page.

In Server mode, widgets and what you type into them (notes, to-dos, bookmarks) are saved on the server, so every device sees the same page. In Static mode they are saved in this browser until you export your config.

Widgets load after the page, one small file each, so a widget you don't use costs nothing. They stop refreshing while the tab is hidden and catch up when you come back.

## Built-in widgets

| Widget | What it shows | Needs |
| --- | --- | --- |
| **Clock** | The time and date, in your time zone or another one | — |
| **Weather** | Current weather and three days ahead. Search for a place or use your location. No API key. | — |
| **Server stats** | CPU, memory, storage, network | Server mode |
| **Temperatures** | The hottest reading of each sensor, with its limit | Server mode |
| **Disks** | Space used on each disk | Server mode |
| **Bookmarks** | A list of links | — |
| **Notes** | A text scratchpad | — |
| **To-do** | A checklist | — |
| **News feed (RSS)** | Latest entries of an RSS or Atom feed | Server mode |
| **Calendar** | Upcoming events from an `.ics` link (repeating events included) | Server mode |
| **Releases** | New releases of the projects you run (public repositories, written `owner/name`) | Server mode |
| **Expiry** | When the certificates of your https services and your domains end (domains are looked up in the public registries) | Server mode |
| **API (no code)** | Numbers or lists from any JSON API (see below) | Server mode |

Widgets marked *Server mode* fetch their data through the Haepage server. That keeps keys private and avoids browser cross-site limits.

Services with an `https://` address also warn on their card when their certificate ends in less than 14 days. That needs no widget.

## The no-code API widget

Point it at any app that answers with JSON and choose what to show. No code.

1. Add **API (no code)** and enter the **address**, for example `http://192.168.1.2/api/stats`.
2. If the API needs a key, add a **header** such as `Authorization: Bearer {{secrets.my_app}}`, then set `my_app` in **Settings → Secrets**. The key stays on the server; the browser only ever sees the name.
3. Pick a **display**:
   - **Stat**: big numbers side by side.
   - **Progress**: bars, each with a maximum (a number or another field).
   - **List** or **Table**: one row per item of a list in the answer. Choose which list.
4. Click the **suggested fields** from the live answer, or type a path yourself: `data.total`, `items[0].name`. Give each field a label and a format: number, percent, bytes, duration, date or text.

### Apps on your own network

For safety, the server won't fetch addresses on your local network unless you allow them. When the widget says the address is private, press **Allow <host>**, or add the host to `settings.proxyAllow` in `haepage.json`:

```json
{ "settings": { "proxyAllow": ["192.168.1.2"] } }
```

Hosts of services already on your page are allowed without this.

### Limits

- Answers must be JSON and at most 2 MB, and arrive within 10 seconds.
- Each widget's answer is kept for its refresh time (at least 5 seconds), so many open tabs don't hammer your app.
- Up to 60 widget requests per minute per browser address.

## In `haepage.json`

Widgets live in the `widgets` list. Settings writes this for you, but you can also edit it by hand:

```json
{
  "widgets": [
    { "id": "clock-1", "type": "clock", "area": "header" },
    { "id": "news", "type": "rss", "title": "News", "area": "sidebar", "refresh": 900,
      "options": { "url": "https://example.com/feed.xml", "limit": 5 } },
    { "id": "stats", "type": "api", "title": "My App", "area": "media", "refresh": 60,
      "options": {
        "url": "http://192.168.1.2/api/stats",
        "headers": { "Authorization": "Bearer {{secrets.my_app}}" },
        "display": "stat",
        "fields": [{ "label": "Users", "path": "users.total", "format": "number" }]
      } }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `id` | Unique, lowercase letters, digits and `-` |
| `type` | `clock`, `weather`, `system`, `temperature`, `disks`, `bookmarks`, `notes`, `todo`, `rss`, `calendar`, `releases`, `expiry` or `api` |
| `title` | Heading on the tile. Leave it out for the widget's own name, or set `""` for none |
| `area` | `header` (top, the default), a group's `id`, or `sidebar` (bottom) |
| `refresh` | Seconds between updates. Defaults: API 60, feeds, calendar and weather 900, releases and expiry 3600 |
| `options` | The widget's own settings. See [the widget API](../widget-api.md) for each type |
