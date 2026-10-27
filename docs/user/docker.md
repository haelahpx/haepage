# Docker labels

Haepage names and groups most containers by itself. Labels let you set anything explicitly, right in your compose files:

```yaml
services:
  movies:
    image: example/media-server
    ports:
      - "8096:8096"
    labels:
      haepage.name: Movies
      haepage.group: media
      haepage.icon: https://example.com/movies.png
      haepage.description: Films and shows
      haepage.url.public: https://movies.example.com
```

| Label | Effect |
| --- | --- |
| `haepage.enable=false` | Never show this container |
| `haepage.id` | A stable id, so your choices survive renames and moves between compose projects |
| `haepage.name` | Card title |
| `haepage.group` | Suggested group (lowercase id, for example `media`) |
| `haepage.icon` | Icon image URL |
| `haepage.description` | Second line on the card |
| `haepage.url` or `haepage.url.lan` | Link used on your home network |
| `haepage.url.tailscale` | Link used over Tailscale |
| `haepage.url.public` | Link used from anywhere else |
| `haepage.port` | Which port to link to, when the container publishes several |
| `haepage.check` | `http` (default), `tcp`, or `none` to turn health checks off |
| `haepage.check.path` | Path for the HTTP health check, for example `/health` |

Without URL labels, Haepage links to `http://<the address you opened Haepage with>:<published port>`. It uses `https` for ports 443 and 8443.

Labels starting with `dash.` are accepted too (`dash.name`, `dash.group`, …), so compose files written for other dashboards keep working.

## Which containers appear

- A container appears if it publishes a port on the host, or has a URL label.
- Ports published only on `127.0.0.1` are skipped, because other devices can't reach them.
- Stopped containers stay on the page, marked as down, until you remove them.
- Haepage's own container is never shown.

## How a container keeps its identity

Your choices (Add, Hide, rename) are remembered by a key that survives a container being recreated:

1. the `haepage.id` label, if set;
2. otherwise the Compose project and service name, for example `docker:media/movies`;
3. otherwise the container name.

Container IDs are never used, because they change every time a container is recreated.
