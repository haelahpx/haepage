# App integrations

Haepage can show live data from your media apps right on the dashboard: who's watching, new requests, downloads and what's coming up. You also get a pop-up the moment someone requests a movie or a download finishes.

This needs Server mode (see [installing](install.md)). Your API keys stay on the Haepage server, in `secrets.json`. Browsers never see them.

| App | On its card | Tile | Pop-ups |
| --- | --- | --- | --- |
| **Jellyfin** | `▶ 2`: people watching | **Now playing**: poster, who, episode, progress, device, direct play or transcoding | (in the Activity feed only) |
| **Jellyseerr** or **Overseerr** | `3 pending` | **Requests**: poster, title, who asked, when | New request, approved, ready to watch |
| **Radarr**, **Sonarr** | `↓ 2` downloading, or `!` when the app reports a problem | **Downloads** (progress and time left), **Coming up** (next 7 days) | Download started, ready to watch, health problems |

An **Activity** tile collects everything that happened, newest first.

## Connecting an app

1. Haepage recognizes these apps when it detects them in Docker. A **Live data** tile then lists the ones you can connect. You can also go to **Settings → Apps**.
2. Click **Connect** and paste the app's API key:

   | App | Where to find the key |
   | --- | --- |
   | Jellyfin | Dashboard → API Keys → **+**. Name it Haepage. |
   | Jellyseerr / Overseerr | Settings → General → API Key |
   | Radarr / Sonarr | Settings → General → API Key |

3. Haepage checks the key with the app before saving it. A wrong key, or an address that turns out to be a different app, is reported straight away.

Services you add by hand can be connected too. Add `"integration": "jellyfin"` (or `"seerr"`, `"radarr"`, `"sonarr"`) to the service in `haepage.json`.

## Instant pop-ups with webhooks

Haepage checks connected apps every 10 to 30 seconds while the dashboard is open, and not at all when nobody is looking. For pop-ups that arrive within a second, even when nobody had the page open, let the app tell Haepage directly:

1. In Haepage, open **Settings → Apps → Manage** for the app and copy the **Webhook URL**.
2. In the app:
   - **Jellyseerr / Overseerr:** Settings → Notifications → Webhook. Turn it on, paste the URL, keep the default JSON payload, and tick *Request Pending Approval*, *Request Approved* and *Request Available*.
   - **Radarr / Sonarr:** Settings → Connect → **+** → Webhook. Paste the URL and tick *On Grab*, *On File Import* and *On Health Issue*.
3. Use the app's **Test** button. "Webhook connected" appears in Haepage's Activity feed.

The URL contains a long random token, which works like a password. Disconnecting the app in Haepage invalidates it.

## Desktop notifications

**Settings → Desktop notifications → Allow** shows the same pop-ups as system notifications while the Haepage tab is in the background.

## Privacy

Who is watching and who requested what is personal. Integration tiles will be hidden from guests once logins and public views arrive (planned). Request and download posters load from the movie database the app uses (TMDB). Jellyfin posters go through your Haepage server, because Jellyfin only serves them with the API key.

## Trying it without the apps

The repository includes fake versions of all four apps for testing:

```sh
go run ./tools/fakemedia
```

It prints four services to add to `haepage.json`. Then connect each one with the key `test-key`. Every 20 seconds something happens: downloads move along, new requests arrive, someone starts watching.
