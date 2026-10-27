# Haepage Widgets

The built-in widgets live in [`web/src/widgets`](../web/src/widgets), one module per type, each loaded only when a widget of that type is on the page. The list of types is in [`registry.js`](../web/src/widgets/registry.js).

- How to use them: [widgets guide](../docs/user/widgets.md)
- The contract (config, options, server endpoints, custom widget manifest): [widget API](../docs/widget-api.md)

## Built-in widgets

| `type` | Widget |
| --- | --- |
| `clock` | Time and date, any time zone |
| `weather` | Current weather and three days ahead, no API key |
| `system` | Server stats: CPU, memory, storage, network |
| `temperature` | Temperatures per sensor |
| `disks` | Space used per disk |
| `bookmarks` | A list of links |
| `notes` | A text scratchpad |
| `todo` | A checklist |
| `rss` | RSS or Atom news feed |
| `calendar` | Upcoming events from an `.ics` link |
| `releases` | New releases of the projects you run |
| `expiry` | Certificate and domain expiry dates |
| `api` | No-code widget for any JSON API: stat, progress, list or table |

## Custom widgets (planned)

Custom widgets will run sandboxed and declare what they need in a `widget.json` manifest ([`schema/widget.schema.json`](../schema/widget.schema.json)):

```json
{
  "name": "Example",
  "id": "example",
  "version": "1.0.0",
  "widget-api": 1,
  "entry": "index.html",
  "permissions": { "network": ["https://api.example.com"], "secrets": ["example_token"], "storage": true }
}
```

See the [widget API](../docs/widget-api.md#custom-widget-manifest-v1-planned) for every field.
