# Haepage Widgets

Built-in and custom widgets directory.

## Built-in Widgets
- `clock`: Time and date display
- `weather`: Weather forecast using open weather APIs
- `systemstats`: CPU, RAM, disk, and temperature stats
- `bookmarks`: Quick link lists
- `notes`: Simple markdown/text scratchpad
- `todo`: Task checklist
- `rss`: RSS feed reader
- `calendar`: ICS calendar feed integration

## Custom Widget Manifest Format
Custom widgets provide an isolated sandboxed interface:
```json
{
  "name": "Widget Name",
  "version": "1.0.0",
  "entry": "index.html"
}
```
