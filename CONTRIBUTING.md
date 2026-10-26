# Contributing to Haepage

Thanks for helping. Haepage is young, so ideas, bug reports, themes and code are all welcome.

## Getting started

You need Go (the current stable release), Node.js 22 or newer, and `make`.

```sh
git clone https://github.com/haelahpx/haepage
cd haepage
make dev        # builds the frontend and runs the server at http://localhost:8080
```

Other targets:

| Command | What it does |
| --- | --- |
| `make build` | Builds the frontend, then a static `./haepage` binary with it embedded |
| `make test` | Go tests (with the race detector) and frontend tests |
| `make lint` | `gofmt`, `go vet`, and schema validation of the example config and themes |
| `make clean` | Removes build output |

## Project layout

| Path | What lives there |
| --- | --- |
| `cmd/haepage/` | The binary's entry point: flags and wiring |
| `internal/` | Server packages (config, discovery, health, proxy, themes, auth, …) |
| `web/` | The Preact frontend. `web/build.mjs` builds it into `web/dist`, which the binary embeds. |
| `themes/` | `base.css` (layout) plus the built-in themes |
| `schema/` | JSON Schemas for the config, themes, widgets and the known-apps list |
| `packaging/` | Docker, systemd, deb and rpm |
| `docs/` | User docs and public API contracts |

## Making a change

1. Open an issue first for anything bigger than a small fix, so we can agree on the approach.
2. Branch from `main`: `feat/<short-name>` or `fix/<short-name>`.
3. Use [Conventional Commits](https://www.conventionalcommits.org/): `feat`, `fix`, `refactor`, `chore`, `docs`, `style`.
4. Run `make lint test` before you push. CI runs the same checks, plus size budgets.
5. Fill in the pull request template.

## Ground rules

- **Stay small.** CI fails if the first-load JavaScript passes 60 KB gzipped or the binary passes 15 MB. Every new Go or npm dependency needs a one-line reason in the PR.
- **Keep public contracts stable.** The config schema, [theme API](docs/theme-api.md) and widget API are versioned. A breaking change needs a new version plus a migration, never an in-place edit.
- **Safe by default.** Anything that can change the host (container actions, shell commands, log access) stays off until an admin turns it on.
- **Never commit secrets.** `secrets.json` is git-ignored. Keep it that way.

## Sharing a theme

Themes are plain CSS. Read the [Theme API](docs/theme-api.md), then open an issue with the **Theme** template to share yours.

## License

By contributing, you agree that your contributions are licensed under the [Apache License 2.0](LICENSE).
