import { useEffect, useRef, useState } from 'preact/hooks';
import {
  Category,
  Food,
  categoryEmoji,
  categoryLabel,
  foodCategories,
  foodIcon,
  normalize,
} from '../api';

interface Props {
  foods: Food[];
  categories: Category[];
  onBack: () => void;
  onPick: (food: Food) => void;
  onQuickAdd: (name: string, categories: string[]) => void;
  onFullForm: () => void;
}

export function SearchPage({ foods, categories, onBack, onPick, onQuickAdd, onFullForm }: Props) {
  const [q, setQ] = useState('');
  const [cats, setCats] = useState<string[]>(['inne']);
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
        .filter((f) => normalize(f.name).includes(nq) || foodCategories(f).some((c) => normalize(c).includes(nq)))
        .sort((a, b) => {
          const sa = normalize(a.name).startsWith(nq) ? 0 : 1;
          const sb = normalize(b.name).startsWith(nq) ? 0 : 1;
          return sa - sb || a.name.localeCompare(b.name, 'pl');
        })
    : [];

  const exact = query !== '' && foods.some((f) => normalize(f.name) === nq);
  const showAdd = query !== '' && !exact;

  function toggleCat(name: string) {
    if (cats.includes(name)) {
      if (cats.length === 1) return; // keep at least one category
      setCats(cats.filter((c) => c !== name));
      return;
    }
    setCats([...cats, name]);
  }

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
          <button class="quick-add-btn" onClick={() => onQuickAdd(q.trim(), cats)}>
            ➕ Dodaj „{q.trim()}”
          </button>
          <div class="chip-grid">
            {categories.map((c) => (
              <button
                key={c.id}
                class={cats.includes(c.name) ? 'chip chip-active chip-sm' : 'chip chip-sm'}
                onClick={() => toggleCat(c.name)}
                title={categoryLabel(c.name)}
              >
                {categoryEmoji(c.name, categories)}
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
            <span class="search-emoji">{foodIcon(f, categories)}</span>
            <div class="search-info">
              <span class="search-name">{f.name}</span>
              <span class="search-cat">
                {foodCategories(f).map(categoryLabel).join(' · ')}
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