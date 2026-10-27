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
| `data-style` | `bento` (tiles, the default) or `simple` (a plain list of groups) |

Style both layouts. In `bento`, groups are tiles (`.hp-bento .hp-group.hp-tile`) and cards sit inside them, so most themes make cards lighter there.

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
| `--surface-low` | Cards inside bento tiles |
| `--bg-image` | Extra page background layer (gradients), drawn on a fixed layer |
| `--radius` ★ | Card corner radius |
| `--radius-sm` | Chips, inputs, small elements |
| `--tile-radius` | Bento tile corner radius |
| `--icon-radius` | Service icon corner radius |
| `--gap` | Space between cards and tiles |
| `--card-pad`, `--tile-pad` | Padding inside a card or tile |
| `--card-min` | Minimum card width |
| `--shadow` | Card shadow |
| `--tile-shadow` | Tile shadow (inset highlights work well here) |
| `--ring-track` | Empty part of meters and bars |
| `--ease` | Default easing curve |
| `--blur` | Backdrop blur radius for glass effects |
| `--font`, `--font-mono` | Font stacks (system fonts by default) |
| `--status-up`, `--status-down`, `--status-unknown` | Health dot colors |
| `--motion` | `1` normally, `0` when the visitor prefers reduced motion. Multiply durations by it. |
| `--wallpaper`, `--wallpaper-blurred` | **Read-only.** Set by Haepage from the wallpaper settings. |

## Classes

| Area | Classes |
| --- | --- |
| Page | `hp-app` · `hp-header` · `hp-header-end` · `hp-title` · `hp-empty` |
| Bento | `hp-bento` · `hp-tile` · `hp-tile--hero` · `hp-tile--wide` · `hp-tile--status` · `hp-hero-clock` · `hp-hero-main` · `hp-hero-eyebrow` · `hp-hero-home` · `hp-hero-actions` · `hp-greeting` |
| Status tile | `hp-tile--status` (with `data-state="up"`, `"down"` or `"idle"`) · `hp-status-big` · `hp-meter` · `hp-meter-segments` · `hp-seg` · `hp-seg--up` · `hp-seg--down` · `hp-seg--unknown` · `hp-meter-bar` · `hp-meter-up` · `hp-meter-down` · `hp-meter-rest` · `hp-status-headline` · `hp-down-list`. Show either the segments (one per service) or the bar; the bar is the default. |
| Server tile | `hp-tile--system` · `hp-metrics` · `hp-metric` · `hp-metric-body` · `hp-metric-top` · `hp-metric-label` · `hp-metric-value` · `hp-metric-detail` · `hp-bar` · `hp-bar-fill` (reads `--avatar-hue`) · `hp-system-foot` |
| Clock | `hp-clock` · `hp-clock-time` · `hp-clock-h` · `hp-clock-sep` · `hp-clock-m` · `hp-clock-ampm` · `hp-clock-date` |
| Search | `hp-search` · `hp-search-icon` · `hp-search-input` · `hp-search-kbd` · `hp-search-results` · `hp-result` · `hp-result--active` · `hp-result-name` · `hp-result-meta` |
| Services | `hp-group` · `hp-group-title` · `hp-group-icon` · `hp-group-name` · `hp-group-sub` · `hp-grid` · `hp-card` · `hp-card-icon` · `hp-avatar` · `hp-card-body` · `hp-card-title` · `hp-card-desc` · `hp-card-chevron` (hidden unless a theme shows it) |
| Status | `hp-status` · `hp-status--up` · `hp-status--down` · `hp-status--unknown` · `hp-status-label` · `hp-status-label--up` · `hp-status-label--down` (hidden unless a theme shows it) · `hp-badge` · `hp-badge--new` |
| Controls | `hp-button` · `hp-button--quiet` · `hp-icon-button` · `hp-link` · `hp-field` · `hp-error` |
| Panels | `hp-banner` · `hp-banner--error` · `hp-drawer` · `hp-drawer-backdrop` · `hp-palette` · `hp-toast` |
| Integrations | `hp-chip` · `hp-chip--live` · `hp-chip--info` · `hp-chip--warn` · `hp-tile--media` · `hp-integrations` (Simple layout row) · `hp-media-list` · `hp-media` · `hp-media--compact` · `hp-media-poster` · `hp-media-poster--empty` · `hp-media-body` · `hp-media-title` · `hp-media-sub` · `hp-media-meta` · `hp-activity` · `hp-activity-title` · `hp-activity-meta` · `hp-connect-list` · `hp-connect-intro` |
| Dialogs and pop-ups | `hp-dialog` · `hp-dialog-backdrop` · `hp-dialog-help` · `hp-dialog-ok` · `hp-copy` · `hp-toasts` · `hp-toast` · `hp-toast-image` · `hp-toast-body` · `hp-toast-title` · `hp-toast-detail` |
| Widgets | `hp-widget` · `hp-widget-title` · `hp-widget-body` |

`hp-avatar` is the colored letter shown when a service has no icon, and `hp-group-icon` is the icon square in a bento group header. Both read `--avatar-hue` (0 to 359), which Haepage sets per service or group.

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

Haepage picks a tier for each device and the visitor can override it. Low-memory, data-saving and reduced-motion devices get `lite`. Others are measured once by frame timing, and the result is remembered in that browser.

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
