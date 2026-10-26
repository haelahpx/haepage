# Theme API v1

This is the public contract for Haepage themes. A theme built against `"theme-api": 1` keeps working on every Haepage release that supports version 1. Breaking changes only ever come with a new version number.

## What a theme is

Either of these:

- **A folder** with `theme.json` (metadata) and `style.css`.
- **A single CSS file** that sets a few variables. This is the quickest way to share a look.

Theme CSS sets **variables** and styles the **`hp-` classes** listed below. Page layout (grid, spacing, structure) belongs to Haepage's base styles. Don't override it, because layout can change between releases.

## `theme.json`

```json
{
  "name": "My Theme",
  "author": "you",
  "theme-api": 1,
  "layout": "grid",
  "accent": "#6750A4",
  "variants": ["light", "dark"],
  "tiers": ["full", "balanced", "lite"]
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Display name |
| `author` | no | Your name or handle |
| `theme-api` | yes | Always `1` for this version |
| `layout` | no | `grid` (the only layout in v1) |
| `accent` | no | Accent color shown in the theme picker |
| `variants` | no | Color schemes the theme supports |
| `tiers` | no | Effect tiers the theme has styles for |

## Attributes on `<html>`

Haepage sets these. Your CSS can target them.

| Attribute | Values |
| --- | --- |
| `data-theme` | The active theme id |
| `data-scheme` | `light`, `dark` |
| `data-fx` | `full`, `balanced`, `lite` |
| `data-mode` | `static`, `server` |

## Variables

★ marks the variables a theme should always set. Every other variable has a sensible default.

| Variable | Purpose |
| --- | --- |
| `--accent` ★ | Brand color |
| `--on-accent` ★ | Text and icons drawn on the accent color |
| `--bg` ★ | Page background (color or gradient) |
| `--surface` ★ | Card background |
| `--surface-variant` ★ | Raised panels, hover states, chips |
| `--on-surface` ★ | Main text |
| `--on-surface-muted` ★ | Secondary text |
| `--border` | Hairlines around cards |
| `--radius` ★ | Card corner radius |
| `--radius-sm` | Icons, chips, small elements |
| `--gap` | Space between cards |
| `--card-pad` | Padding inside a card |
| `--card-min` | Minimum card width |
| `--shadow` | Card shadow |
| `--blur` | Backdrop blur radius for glass effects |
| `--font`, `--font-mono` | Font stacks (system fonts by default) |
| `--status-up`, `--status-down`, `--status-unknown` | Health dot colors |
| `--motion` | `1` normally, `0` when the visitor prefers reduced motion. Multiply durations by it. |
| `--wallpaper`, `--wallpaper-blurred` | **Read-only.** Set by Haepage from the wallpaper settings. |

## Classes

`hp-app` · `hp-header` · `hp-title` · `hp-clock` · `hp-search` · `hp-search-input` · `hp-search-results` · `hp-result` · `hp-result--active` · `hp-group` · `hp-group-title` · `hp-grid` · `hp-card` · `hp-card-icon` · `hp-card-body` · `hp-card-title` · `hp-card-desc` · `hp-status` · `hp-status--up` · `hp-status--down` · `hp-status--unknown` · `hp-badge` · `hp-badge--new` · `hp-widget` · `hp-widget-title` · `hp-widget-body` · `hp-palette` · `hp-toast` · `hp-empty`

## Light and dark

Put light values on `:root` and dark values under `[data-scheme="dark"]`:

```css
:root {
  --accent: #e5484d;
  --bg: #fafafa;
  --surface: #ffffff;
  --on-surface: #1a1a1f;
}
:root[data-scheme="dark"] {
  --bg: #111113;
  --surface: #1a1a1f;
  --on-surface: #ededef;
}
```

## Effect tiers

Haepage picks a tier for each device and the visitor can override it:

- **full**: every effect.
- **balanced**: blur capped at about 12px, fewer animations.
- **lite**: no blur and no shadows. Haepage forces `--blur: 0` and `--shadow: none`.

Make sure the theme still looks right on `lite`, because many phones will use it.

## Performance rules

These keep heavy themes smooth. The theme linter checks them.

- Animate only `transform` and `opacity`. Never animate `width`, `height`, `top`, `left`, `filter` or `backdrop-filter`.
- Keep live `backdrop-filter` to small areas. For frosted glass over a wallpaper, prefer `--wallpaper-blurred` as a background.
- Keep CSS under about 50 KB.
- Don't `@import` CSS from other sites.
- Use the system font stack, or subset a web font and use `font-display: swap`.
