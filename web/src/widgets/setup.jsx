import { useState } from "preact/hooks";

// A one-field setup form shared by the feed, calendar and release widgets.
// `multiline` takes one value per line.
export function SetupForm({ label, placeholder, help, initial = "", multiline = false, onSave, onCancel }) {
  const [value, setValue] = useState(initial);
  const Input = multiline ? "textarea" : "input";
  return (
    <form
      class="hp-w-setup"
      onSubmit={(e) => {
        e.preventDefault();
        if (value.trim()) onSave(value.trim());
      }}
    >
      <label>
        <span>{label}</span>
        <Input placeholder={placeholder} rows={multiline ? 4 : undefined} value={value} onInput={(e) => setValue(e.currentTarget.value)} />
      </label>
      {help && <small>{help}</small>}
      <div class="hp-w-api-actions">
        <button type="submit" class="hp-button" disabled={!value.trim()}>
          Save
        </button>
        {onCancel && (
          <button type="button" class="hp-button hp-button--quiet" onClick={onCancel}>
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}

export function NeedsServer() {
  return <p class="hp-widget-empty">This widget needs Haepage's Server mode, which fetches the data for it.</p>;
}
