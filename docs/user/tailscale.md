# Tailscale

Haepage works well with [Tailscale](https://tailscale.com): reach your page from anywhere on your tailnet, give it an `https://` address, and see your devices on the page.

## Open Haepage over Tailscale with https

Run this on the machine where Haepage runs (Haepage listens on port 8080 by default):

```sh
tailscale serve --bg 8080
```

Haepage is now at `https://<machine>.<tailnet>.ts.net`, for example `https://homelab.tail1234.ts.net`. Tailscale gets and renews the certificate. If the command asks you to, turn on **MagicDNS** and **HTTPS certificates** in the Tailscale admin console first.

The https address is worth having: browsers only let Haepage [install as an app and keep working offline](install.md#install-as-an-app-and-offline-use) on https pages.

With Docker, publish Haepage's port on the host as usual (the compose file does) and run `tailscale serve` on the host.

To stop serving: `tailscale serve reset` (this clears everything `tailscale serve` was sharing on that machine).

## Smart links

A service can have up to three addresses: `lan`, `tailscale` and `public`. Haepage opens the one that fits how you reached the page:

| You opened Haepage with | It opens |
| --- | --- |
| A Tailscale name (`*.ts.net`) or a Tailscale IP (`100.64.x.x` to `100.127.x.x`) | `tailscale`, then `lan`, then `public` |
| A home-network address (`192.168.x.x`, `10.x.x.x`, `localhost`, `*.local`, `*.lan`, …) | `lan`, then `tailscale`, then `public` |
| Anything else (your public domain) | `public`, then `tailscale`, then `lan` |

```json
{
  "id": "media-server",
  "name": "Media Server",
  "urls": {
    "lan": "http://192.168.1.10:8096",
    "tailscale": "https://media.tail1234.ts.net"
  }
}
```

Services Haepage detects by itself use `{host}`, the address you opened the page with, so their links work from home and over Tailscale without setting anything. With Docker labels, `haepage.url.tailscale` sets the Tailscale link ([Docker guide](docker.md)).

## Devices tile

When Haepage runs on a machine with Tailscale, a **Devices** tile lists the machines on your tailnet: this server first, then the devices that are online, then the offline ones with when they were last seen. Each row shows the operating system, the Tailscale IP, and whether the connection is relayed or the device is an exit node.

Haepage reads this from Tailscale on the same machine (`/var/run/tailscale/tailscaled.sock`); it needs no API key and changes nothing.

- **Installed with the script or as a binary:** the tile appears by itself.
- **Docker:** mount the socket by uncommenting this line in the compose file:

  ```yaml
  - /var/run/tailscale/tailscaled.sock:/var/run/tailscale/tailscaled.sock
  ```

- **Socket somewhere else:** `--tailscale-socket /path/to/tailscaled.sock` or `HAEPAGE_TAILSCALE_SOCKET`.
- **To hide the tile:** start Haepage with `--no-tailscale`.

Haepage never sends device keys or your devices' public internet addresses to the browser, only their names, operating systems and Tailscale IPs.

## Keep it private for now

Haepage doesn't have logins yet. Anyone who can open the page can see your services, server stats and, with the Devices tile, your device names and Tailscale IPs. Open it on your home network or over Tailscale, and don't put it on a public domain until logins arrive.

`tailscale serve` is safe here: it only shares Haepage with your tailnet. **Don't** use `tailscale funnel`, which puts it on the public internet.
