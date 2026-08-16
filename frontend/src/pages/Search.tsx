import { useEffect, useRef, useState } from 'preact/hooks';
import {
  CATEGORY_ORDER,
  Food,
  categoryEmoji,
  categoryLabel,
  foodEmoji,
  normalize,
} from '../api';

interface Props {
  foods: Food[];
  onBack: () => void;
  onPick: (food: Food) => void;
  onQuickAdd: (name: string, category: string) => void;
  onFullForm: () => void;
}

export function SearchPage({ foods, onBack, onPick, onQuickAdd, onFullForm }: Props) {
  const [q, setQ] = useState('');
  const [cat, setCat] = useState('inne');
  const inputRef = useRef<HTMLInputElement>(null);

  // Focus + pop the keyboard right after the page mounts.
  useEffect(() => {
    const t = window.setTimeout(() => inputRef.current?.focus(), 50);
    return () => window.clearTimeout(t);
  }, []);

  const query = q.trim().toLowerCase();
  const nq = normalize(query);
  const matches = query
    ? foods
        .filter((f) => normalize(f.name).includes(nq) || normalize(f.category).includes(nq))
        .sort((a, b) => {
          const sa = normalize(a.name).startsWith(nq) ? 0 : 1;
          const sb = normalize(b.name).startsWith(nq) ? 0 : 1;
          return sa - sb || a.name.localeCompare(b.name, 'pl');
        })
    : [];

  const exact = query !== '' && foods.some((f) => normalize(f.name) === nq);
  const showAdd = query !== '' && !exact;

  return (
    <div class="page search-page">
      <div class="search-bar">
        <button class="btn-ghost btn-back" onClick={onBack} aria-label="wstecz">
          ←
        </button>
        <input
          class="input input-xl search-input"
          placeholder="Czego dziś spróbował?"
          value={q}
          onInput={(e) => setQ((e.target as HTMLInputElement).value)}
          ref={inputRef}
          autocapitalize="none"
          autoComplete="off"
          enterkeyhint="search"
          autoFocus
        />
      </div>

      {showAdd && (
        <div class="quick-add">
          <button class="quick-add-btn" onClick={() => onQuickAdd(q.trim(), cat)}>
            ➕ Dodaj „{q.trim()}”
          </button>
          <div class="chip-grid">
            {CATEGORY_ORDER.map((c) => (
              <button
                key={c}
                class={cat === c ? 'chip chip-active chip-sm' : 'chip chip-sm'}
                onClick={() => setCat(c)}
                title={categoryLabel(c)}
              >
                {categoryEmoji(c)}
              </button>
            ))}
          </div>
          <button class="link-btn" onClick={onFullForm}>
            pełny formularz (notatki, cel) →
          </button>
        </div>
      )}

      {!query && (
        <p class="search-hint muted">
          Szybkie logowanie: wpisz lub wybierz jedzenie poniżej, potem tylko ocena. 🍼
          <br />
          Polskie znaki niepotrzebne — „wolowina” znajdzie „wołowinę”.
        </p>
      )}

      {query && matches.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍴</div>
          <p>Nic nie znaleziono{showAdd ? ' — dodaj wyżej!' : '.'}</p>
        </div>
      )}

      <div class="search-results">
        {matches.map((f) => (
          <div class="search-row" key={f.id} onClick={() => onPick(f)}>
            <span class="search-emoji">{foodEmoji(f.name, f.category)}</span>
            <div class="search-info">
              <span class="search-name">{f.name}</span>
              <span class="search-cat">
                {categoryLabel(f.category)}
                {f.tries > 0 ? ` · ${f.tries} ${f.tries === 1 ? 'próba' : f.tries < 5 ? 'próby' : 'prób'}` : ''}
              </span>
            </div>
            <span class="search-tries">
              {f.tries >= f.target ? '🎉' : `${f.tries}/${f.target}`}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}