# Arrange your page

Move cards, groups and widgets right on the page, without editing `haepage.json`.

## Edit mode

Press the **pencil** button next to Settings. Movable items get a dashed outline and a bar at the top explains what to do. Press **Done** (or Esc) when you're finished.

- **Mouse:** drag a card, a group (by its title or any empty space in it) or a widget, and drop it where the line shows.
- **Touch:** press and hold for a moment, then drag. A quick swipe still scrolls the page.
- **Keyboard:** Tab to an item, then use the arrow keys to move it within its list, and `[` or `]` to move a card or widget to the previous or next group. A screen reader announces each move.

Cards don't open while you're editing, so you can grab them safely.

## What you can move

- **Cards** within a group, or into another group. A card you drag out of **New** is added to that group.
- **Groups**, to change their order. **New** always stays first and **Other** last.
- **Widgets** within the top or bottom of the page, or into a group. Settings → Widgets can also place them.

Detected services can't be dropped into **Other**: without a group of their own they would go back to the one Haepage suggested.

## Where it's saved

Every move saves at once, like any other setting:

- **Server mode:** in `haepage.json` on the server, for everyone.
- **Static mode:** in this browser, until you download `haepage.json` (the banner at the top offers it).

In the file, group order is each group's `order`. Card order is `order` on the service, or on its entry in `overrides` for services Haepage detected. Widget order is their order in `widgets`, together with their `area`.
