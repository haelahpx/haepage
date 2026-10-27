import { useState } from "preact/hooks";
import { safeUrl } from "./api.js";

// A list of links saved in haepage.json (options.links: [{name, url}]).
export default function Bookmarks({ options, setOptions }) {
  const links = options.links ?? [];
  const [adding, setAdding] = useState(links.length === 0);
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const valid = safeUrl(url.includes("://") ? url : `https://${url}`);

  return (
    <div class="hp-w-bookmarks">
      <ul>
        {links.map((l, i) => {
          const href = safeUrl(l.url);
          return (
            <li key={`${i}-${l.url}`}>
              {href ? <a href={href}>{l.name || new URL(href).host}</a> : <span>{l.name}</span>}
              <button
                type="button"
                class="hp-w-remove"
                aria-label={`Remove ${l.name || l.url}`}
                onClick={() => setOptions({ links: links.filter((_, j) => j !== i) })}
              >
                ×
              </button>
            </li>
          );
        })}
      </ul>
      {adding ? (
        <form
          class="hp-w-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!valid) return;
            setOptions({ links: [...links, { name: name.trim(), url: valid }] });
            setName("");
            setUrl("");
            setAdding(false);
          }}
        >
          <input placeholder="Name" aria-label="Link name" value={name} onInput={(e) => setName(e.currentTarget.value)} />
          <input placeholder="example.com" aria-label="Link address" value={url} onInput={(e) => setUrl(e.currentTarget.value)} />
          <button type="submit" class="hp-button" disabled={!valid}>
            Add
          </button>
        </form>
      ) : (
        <button type="button" class="hp-link-button" onClick={() => setAdding(true)}>
          Add a link
        </button>
      )}
    </div>
  );
}
