import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { hostOf } from "../links.js";
import { parseQuery, searchUrl } from "../search/engines.js";
import { search } from "../search/index.js";

const isTyping = (el) => el?.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(el?.tagName);

function open(url, newTab) {
  if (newTab) window.open(url, "_blank", "noopener");
  else window.location.assign(url);
}

// Search box: instant local results, then a web search with the default
// engine or a bang. Keys: / focus, arrows move, Enter opens,
// Ctrl/Cmd+Enter opens in a new tab, Esc clears.
export function Search({ entries, engines, defaultEngine }) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const input = useRef(null);

  useEffect(() => {
    // Skip autofocus on touch screens, where it would pop up the keyboard.
    if (!window.matchMedia("(pointer: coarse)").matches) input.current?.focus();
    const onKey = (e) => {
      if (e.key === "/" && !isTyping(e.target) && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        input.current?.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const items = useMemo(() => {
    const parsed = parseQuery(query, engines);
    if (!parsed.terms && !parsed.bang) return [];
    const local = parsed.bang ? [] : search(entries, parsed.terms);
    const engine = parsed.engine ?? defaultEngine;
    return [...local, { kind: "web", id: "web", engine, terms: parsed.terms, url: searchUrl(engine, parsed.terms) }];
  }, [query, entries, engines, defaultEngine]);

  const onKeyDown = (e) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (items.length ? (a + step + items.length) % items.length : 0));
    } else if (e.key === "Enter" && items[active]) {
      e.preventDefault();
      open(items[active].url, e.ctrlKey || e.metaKey);
    } else if (e.key === "Escape") {
      setQuery("");
      setActive(0);
      input.current?.blur();
    }
  };

  return (
    <div class="hp-search" role="search">
      <svg class="hp-search-icon" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
        <circle cx="10.5" cy="10.5" r="6.5" fill="none" stroke="currentColor" stroke-width="2.2" />
        <path d="M15.5 15.5 21 21" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" />
      </svg>
      <input
        ref={input}
        class="hp-search-input"
        type="search"
        value={query}
        placeholder="Search services or the web"
        title={`Web searches use ${defaultEngine.name}. Type !g, !yt, … for other engines.`}
        aria-label="Search"
        role="combobox"
        aria-expanded={items.length > 0}
        aria-controls="hp-search-results"
        aria-activedescendant={items.length ? `hp-result-${active}` : undefined}
        autocomplete="off"
        spellcheck={false}
        onInput={(e) => {
          setQuery(e.currentTarget.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
      />
      <kbd class="hp-search-kbd" aria-hidden="true">
        /
      </kbd>
      {items.length > 0 && (
        <ul id="hp-search-results" class="hp-search-results" role="listbox">
          {items.map((item, i) => (
            <li
              key={item.kind + item.id}
              id={`hp-result-${i}`}
              role="option"
              aria-selected={i === active}
              class={i === active ? "hp-result hp-result--active" : "hp-result"}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                open(item.url, e.ctrlKey || e.metaKey || e.button === 1);
              }}
            >
              {item.kind === "web" ? (
                <span class="hp-result-name">
                  {item.terms ? `Search ${item.engine.name} for “${item.terms}”` : `Open ${item.engine.name}`}
                </span>
              ) : (
                <>
                  <span class="hp-result-name">{item.name}</span>
                  <span class="hp-result-meta">{item.group ?? hostOf(item.url)}</span>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
