# Static mode

Static mode is Haepage as a folder of plain files. It needs no Haepage server, only something that can serve files. You get your services, groups, search, themes and smart links. Features that need a server (auto-detection, saving for everyone, secrets in widgets) light up later if you move to Server mode.

## Install

1. Download `haepage-static-<version>.zip` from the releases page and unzip it. You get a `haepage/` folder.
2. Put the folder's contents anywhere a web server can serve them: the web root of a server you already run, a folder on your NAS, or any static file host. A sub-folder such as `https://example.com/home/` works too.
3. Edit `haepage.json` in that folder (see below) and open the page.

To try it on your own computer first:

```sh
cd haepage
python3 -m http.server 8080
# open http://localhost:8080
```

Opening `index.html` straight from disk (`file://`) won't work, because browsers block the request that loads `haepage.json`.

## Add your services

Everything lives in `haepage.json`, next to `index.html`:

```json
{
  "version": 1,
  "settings": { "title": "Home Lab", "theme": "tonal", "search": "duckduckgo" },
  "groups": [
    { "id": "media", "name": "Media", "order": 1 },
    { "id": "network", "name": "Network", "order": 2 }
  ],
  "services": [
    {
      "id": "media-server",
      "name": "Media Server",
      "group": "media",
      "description": "Movies and shows",
      "urls": {
        "lan": "http://192.168.1.10:8096",
        "tailscale": "http://server.example.ts.net:8096",
        "public": "https://media.example.com"
      }
    },
    {
      "id": "router",
      "name": "Router",
      "group": "network",
      "icon": "https://192.168.1.1/favicon.ico",
      "urls": { "lan": "http://192.168.1.1" }
    }
  ],
  "bookmarks": [{ "name": "Mail", "url": "https://mail.example.com" }]
}
```

Service fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | yes | Lowercase letters, numbers and dashes. Must be unique. |
| `name` | yes | Shown on the card |
| `urls` | yes | At least one of `lan`, `tailscale`, `public` (see smart links) |
| `group` | no | A group `id`. A group that isn't in `groups` gets its own section, named after the id. Services without a group go to **Other**. |
| `description` | no | Second line on the card. Defaults to the link's address. |
| `icon` | no | An image URL. Without one, the card shows a colored letter. |
| `tags` | no | Extra words that search should match |
| `check` | no | `{ "type": "none" }` turns off the health dot for this service |

The full format is described by the JSON Schema in `schema/haepage.schema.json` in the repository. Editors that support JSON Schema can autocomplete the file with it.

## Smart links

Each service can have up to three addresses. Haepage opens the one that fits how you reached the page:

| You opened Haepage via | It opens first |
| --- | --- |
| A Tailscale name (`*.ts.net`) or a Tailscale IP (`100.64.x.x` to `100.127.x.x`) | `tailscale`, then `lan`, then `public` |
| A home-network address (`192.168.x.x`, `10.x.x.x`, `localhost`, `*.local`, `*.lan`, …) | `lan`, then `tailscale`, then `public` |
| Anything else (your public domain) | `public`, then `tailscale`, then `lan` |

Hover a card to see every address it has.

## Search

The search box is focused when the page opens (except on touch screens).

| Key | Action |
| --- | --- |
| `/` | Jump to the search box |
| `↑` `↓` | Move through results |
| `Enter` | Open the highlighted result |
| `Ctrl`+`Enter` (`Cmd`+`Enter` on macOS) | Open it in a new tab |
| `Esc` | Clear the search |

Your services and bookmarks show up first. The last result always searches the web with your default engine.

**Bangs** send a search to a specific engine. Put them at the start or the end: `!yt lo-fi music` or `lo-fi music !yt`.

| Bang | Engine |
| --- | --- |
| `!d` | DuckDuckGo |
| `!g` | Google |
| `!b` | Brave Search |
| `!w` | Wikipedia |
| `!yt` | YouTube |
| `!gh` | GitHub |

Add your own in `haepage.json`:

```json
"engines": [
  { "id": "maps", "name": "Maps", "prefix": "m", "url": "https://www.openstreetmap.org/search?query=%s" }
]
```

`%s` is replaced by your search. Using the `id` of a built-in engine replaces it, for example to change its bang.

## Health dots

Every minute, while the tab is visible, Haepage checks whether each service answers. A green dot means it answered, red means it didn't, and grey means it couldn't be checked.

Static mode runs these checks from your browser, which has limits:

- It can tell whether a service answers, not whether it answers *correctly*.
- A page served over `https://` can't check `http://` services, because browsers block that. Those show grey.

Server mode does full checks from the server and has neither limit.

## Settings

The gear button opens Settings: layout, title, theme, light or dark, effects, default search engine and custom CSS.

**Layout** is either **Bento** (the default: a large tile with the clock, greeting and search, a status tile with a ring showing how many services are up, and one tile per group) or **Simple** (a header, the search bar and plain rows of cards). Set it in the file with `"settings": { "style": "bento" }` or `"simple"`.

Static mode can't write to `haepage.json` on your server, so changes are saved **in your browser**. A banner reminds you about it:

- **Download haepage.json** saves your changes as a file. Replace the one on your server to keep them for every browser and device.
- **Discard** drops the browser copy and goes back to the file.

**Import** in Settings loads a `haepage.json` from your computer, for example one exported from another device.

## Themes

Pick **Tonal** or **Glass** in Settings, or choose **From a link…** and paste an `https://` link to a theme's CSS file. Static mode fetches that file in your browser, so the site hosting it must allow cross-origin requests. Making your own theme is covered in the [Theme API](https://github.com/haelahpx/haepage/blob/main/docs/theme-api.md).

## Moving to Server mode later

Server mode reads the same `haepage.json`. Copy the file into the server's data folder and everything you set up carries over.
