# Themes

Haepage has two built-in themes, **Tonal** and **Glass**, each with a light and a dark look. Pick one in **Settings → Theme**. To make your own, see the [theme API](../theme-api.md).

## Effects

**Settings → Effects** sets how much animation and blur a theme uses:

- **Automatic** (the default): low-memory, data-saving and reduced-motion devices get Lite. Other devices are timed once, the first time they open the page, and the result is remembered in that browser. Settings shows which tier the device got. **Check again** times it again.
- **Full**, **Balanced** or **Lite**: always use that tier.

## Use a theme from a link

A theme can be a single CSS file on any `https://` address.

- **In Settings:** choose **Theme → From a link…** and paste the address.
- **From a shared link:** open your Haepage address with `?theme=` and the theme's address, for example
  `https://home.example.com/?theme=https%3A%2F%2Fthemes.example.com%2Fred.css`.
  Haepage asks before it changes anything: **Use theme** or **Not now**.

To share the theme you use, open Settings and choose **Copy share link**. Anyone with their own Haepage can open that link on their install.

### Server mode

The server fetches the file, checks it and keeps a copy in `data/cache/themes/`. Every browser then loads the theme from your server, and the theme stays the same even if the file at the link changes later. To get the new version, choose **Update** in Settings.

For safety, the server refuses theme links that point at this machine or your local network. To allow a host on your network, add it to `settings.proxyAllow` in `haepage.json`:

```json
{ "settings": { "proxyAllow": ["themes.lan"] } }
```

Hosts of services already in your config are allowed without this.

A theme file is refused if it isn't CSS (for example a web page), is larger than 512 KB, or uses `@import`.

### Static mode

The browser fetches the file itself, so the site hosting it must allow other pages to load its files (CORS). Most file and code hosts do. If it doesn't, Haepage shows an error and keeps your current theme.
