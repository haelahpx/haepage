import { useState } from "preact/hooks";
import { addItem } from "./lib.js";

// A checklist saved in haepage.json (options.items: [{id, text, done}]).
export default function Todo({ options, setOptions }) {
  const items = options.items ?? [];
  const [draft, setDraft] = useState("");
  const set = (next) => setOptions({ items: next });
  const done = items.filter((i) => i.done).length;

  return (
    <div class="hp-w-todo">
      <ul>
        {items.map((item) => (
          <li key={item.id} class={item.done ? "is-done" : ""}>
            <label>
              <input
                type="checkbox"
                checked={item.done}
                onChange={() => set(items.map((i) => (i.id === item.id ? { ...i, done: !i.done } : i)))}
              />
              <span>{item.text}</span>
            </label>
            <button
              type="button"
              class="hp-w-remove"
              aria-label={`Remove ${item.text}`}
              onClick={() => set(items.filter((i) => i.id !== item.id))}
            >
              ×
            </button>
          </li>
        ))}
      </ul>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          set(addItem(items, draft));
          setDraft("");
        }}
      >
        <input placeholder="Add a task" aria-label="New task" value={draft} onInput={(e) => setDraft(e.currentTarget.value)} />
      </form>
      {done > 0 && (
        <button type="button" class="hp-link-button hp-w-todo-clear" onClick={() => set(items.filter((i) => !i.done))}>
          Clear {done} done
        </button>
      )}
    </div>
  );
}
