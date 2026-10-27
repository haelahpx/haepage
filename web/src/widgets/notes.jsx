// A plain-text note saved in haepage.json (options.text).
export default function Notes({ options, setOptions }) {
  return (
    <textarea
      class="hp-w-notes"
      rows={5}
      placeholder="Write something…"
      aria-label="Notes"
      value={options.text ?? ""}
      onInput={(e) => setOptions({ text: e.currentTarget.value })}
    />
  );
}
