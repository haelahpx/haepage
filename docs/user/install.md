# Installing Haepage (Server mode)

Server mode is one small program. It serves the dashboard, saves your settings for every browser, finds your services by itself, and checks whether they're up. If you only want a page of links, see [Static mode](static-mode.md).

> Packages and images are published with the first release. Until then, build from source (at the end of this page).

## Option 1: Docker Compose

The recommended setup, using [`packaging/docker/compose.yaml`](../../packaging/docker/compose.yaml):

```sh
mkdir haepage && cd haepage
curl -fsSLO https://raw.githubusercontent.com/haelahpx/haepage/main/packaging/docker/compose.yaml
docker compose up -d
```

Open `http://<your-server>:8080`. Your running containers appear in **New** within a second or two. **Add** puts one on the dashboard and **Hide** removes it for good.

How it is set up, and why:

- **Haepage never touches the Docker socket.** A small socket proxy sits in between and only allows reading containers and events. Mounting the socket read-only would not be enough, because it doesn't stop write calls to the Docker API.
- **It runs as a non-root user** on a minimal image (about 10 MB).
- **Settings live in a Docker volume** (`haepage-data`), mounted at `/data`.
- **Health checks go through the host.** `HAEPAGE_CHECK_HOST: host.docker.internal` lets Haepage reach the ports your containers publish on the machine.
- **Port discovery is off**, because inside a container it would only see the container itself. Docker discovery covers containers.

## Option 2: Install script (systemd)

On Debian, Ubuntu, Fedora or any systemd Linux, on amd64, arm64 or armv7 (for example a Raspberry Pi):

```sh
curl -fsSL https://raw.githubusercontent.com/haelahpx/haepage/main/packaging/install.sh | sudo sh
```

The script:

1. Downloads the release for your CPU and checks its SHA-256 checksum.
2. Installs the program to `/usr/local/bin/haepage`.
3. Creates an unprivileged `haepage` user. If a `docker` group exists, it adds the user to it so Docker discovery works.
4. Installs a sandboxed systemd service and starts it.

| What | Where |
| --- | --- |
| Your config | `/var/lib/haepage/haepage.json` |
| Service settings | `/etc/haepage/env` |
| Logs | `journalctl -u haepage -f` |
| Restart | `sudo systemctl restart haepage` |

Want a different port? Run `curl ... | sudo HAEPAGE_PORT=9000 sh`, or edit `/etc/haepage/env` and restart.

## Option 3: The plain binary

Download the archive for your CPU from the releases page, unpack it, and run:

```sh
./haepage --data ./data
```

## Settings

Every setting is a flag or an environment variable:

| Flag | Environment | Default | What it does |
| --- | --- | --- | --- |
| `--addr` | `HAEPAGE_ADDR` | `:8080` | Address and port to listen on |
| `--data` | `HAEPAGE_DATA` | `./data` | Folder for `haepage.json` and other data |
| `--config` | `HAEPAGE_CONFIG` | `<data>/haepage.json` | Config file |
| `--docker` | `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker socket, or `tcp://…` for a socket proxy |
| `--check-host` | `HAEPAGE_CHECK_HOST` | `127.0.0.1` | Host that health checks use for services on this machine |
| `--no-docker` | | | Turn off Docker discovery |
| `--no-ports` | | | Turn off listening-port discovery |
| `--no-health` | | | Turn off server-side health checks |
| `--no-system` | | | Turn off the server stats tile |
| `--metrics` | `HAEPAGE_METRICS=true` | off | Serve metrics for monitoring tools at `/metrics` ([Metrics](metrics.md)) |
| `--log-level` | `HAEPAGE_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `--log-format` | `HAEPAGE_LOG_FORMAT` | `text` | `text` or `json` |

## Install as an app, and offline use

Haepage can be installed like an app on phones and computers (in Chrome or Edge: the install button in the address bar; on iPhone: Share → Add to Home Screen). It opens in its own window without browser bars.

It also keeps working when the server can't be reached: the page shows what the server last sent, with a notice, and catches up by itself when the server is back. Changes can't be saved while it's offline.

Both need the page to be opened over **https://** (or on `localhost`); browsers don't allow them on plain `http://` addresses. The easiest way at home is `tailscale serve`, which gives Haepage an https address on your tailnet ([Tailscale guide](tailscale.md)).

## How detection works

- **Docker containers** with a published port appear on their own. Haepage follows Docker's event stream, so a container you start shows up within about a second, and one you stop turns red straight away.
- **Other web servers** listening on the machine (outside Docker) are found by checking listening ports every 15 seconds. Only ports that answer HTTP are shown.
- **Names, icons and groups** come from a built-in list of common self-hosted apps, from Docker labels (see [Docker labels](docker.md)), or from the container name.
- **New services wait in New** and never rearrange your dashboard. To have them placed straight into their suggested group, set `"settings": { "inbox": "auto" }`.
- **Detection never edits your config file.** Only your clicks do (Add, Hide, settings).

Links to detected services use the address you opened Haepage with. So the same card works from your home network and over Tailscale.

## Building from source

You need Go, Node.js 22 or newer, and `make`:

```sh
git clone https://github.com/haelahpx/haepage
cd haepage
make build                 # builds the page, then the program with the page inside it
./haepage --data ./data    # http://localhost:8080
```

To build the Docker image: `docker build -f packaging/docker/Dockerfile -t haepage .`
