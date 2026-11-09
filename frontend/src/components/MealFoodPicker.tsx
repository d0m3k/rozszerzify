import { useEffect, useState } from 'preact/hooks';
import { Category, Food, categoryEmoji, categoryLabel, foodCategories, foodIcon, normalize } from '../api';

interface Props {
  foods: Food[];
  categories: Category[];
  selected: number[];
  onChange: (ids: number[]) => void;
  /**
   * When provided, the picker offers "＋ Nowy produkt" right here, so a missing
   * ingredient does not force a detour through the add-food page (and losing
   * the half-filled meal form).
   */
  onCreateFood?: (input: { name: string; category: string }) => Promise<Food>;
}

// Multi-select list of existing foods used to build a meal. Selected foods
// are listed first so the meal composition is always visible.
export function MealFoodPicker({ foods, categories, selected, onChange, onCreateFood }: Props) {
  const [q, setQ] = useState('');
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState('');
  const [cat, setCat] = useState(categories[0]?.name ?? 'owoce');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  // Foods created here, kept until the parent's refreshed list catches up.
  const [extra, setExtra] = useState<Food[]>([]);
  const query = normalize(q.trim());

  // Categories arrive asynchronously; keep the chosen one valid.
  useEffect(() => {
    if (categories.length > 0 && !categories.some((c) => c.name === cat)) setCat(categories[0].name);
  }, [categories, cat]);

  const known = [...foods, ...extra.filter((e) => !foods.some((f) => f.id === e.id))];

  function toggle(id: number) {
    onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);
  }

  const matches = known
    .filter((f) => {
      if (!query) return true;
      return normalize(f.name).includes(query) || foodCategories(f).some((c) => normalize(c).includes(query));
    })
    .sort((a, b) => {
      const sa = normalize(a.name).startsWith(query) ? 0 : 1;
      const sb = normalize(b.name).startsWith(query) ? 0 : 1;
      return sa - sb || a.name.localeCompare(b.name, 'pl');
    });

  const selectedFoods = selected
    .map((id) => known.find((f) => f.id === id))
    .filter((f): f is Food => !!f);

  function openNew() {
    setDraft(q.trim());
    setError('');
    setAdding(true);
  }

  function select(id: number) {
    if (!selected.includes(id)) onChange([...selected, id]);
  }

  async function submitNew(e?: Event) {
    e?.preventDefault();
    const name = draft.trim();
    if (!name || busy || !onCreateFood) return;
    setBusy(true);
    setError('');
    try {
      const created = await onCreateFood({ name, category: cat });
      setExtra((prev) => [...prev, created]);
      select(created.id);
      setDraft('');
      setQ('');
      setAdding(false);
    } catch (err: unknown) {
      // The product may already exist (409): pick it instead of failing.
      const existing = known.find((f) => normalize(f.name) === normalize(name));
      if (existing) {
        select(existing.id);
        setDraft('');
        setQ('');
        setAdding(false);
      } else {
        setError(err instanceof Error ? err.message : 'Nie udało się dodać produktu');
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div class="meal-picker">
      {selectedFoods.length > 0 && (
        <div class="meal-selected">
          {selectedFoods.map((f) => (
            <button type="button" key={f.id} class="chip chip-active meal-selected-chip" onClick={() => toggle(f.id)}>
              {foodIcon(f, categories)} {f.name} <span class="meal-selected-x">×</span>
            </button>
          ))}
        </div>
      )}
      <input
        class="input"
        value={q}
        onInput={(e) => setQ((e.target as HTMLInputElement).value)}
        placeholder="Szukaj produktu do dania…"
        autocapitalize="none"
        autoComplete="off"
        onKeyDown={(e) => {
          // The picker lives inside the meal <form>: never submit the meal
          // because someone pressed Enter while filtering.
          if (e.key === 'Enter') e.preventDefault();
        }}
      />

      {onCreateFood && !adding && (
        <button type="button" class="chip chip-new meal-new-open" onClick={openNew}>
          ＋ Nowy produkt{q.trim() ? ` „${q.trim()}”` : ''}
        </button>
      )}

      {onCreateFood && adding && (
        <div class="meal-new-food">
          <div class="cat-new-row">
            <input
              class="input"
              value={draft}
              onInput={(e) => setDraft((e.target as HTMLInputElement).value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  void submitNew();
                }
              }}
              placeholder="np. kalarepa, amarantus…"
              autocapitalize="none"
              autoComplete="off"
              autoFocus
            />
            <button class="btn-primary" type="button" onClick={() => void submitNew()} disabled={busy || !draft.trim()}>
              {busy ? '…' : 'Dodaj'}
            </button>
            <button
              type="button"
              class="btn-ghost"
              onClick={() => {
                setAdding(false);
                setDraft('');
                setError('');
              }}
            >
              Anuluj
            </button>
          </div>
          <div class="chip-grid">
            {categories.map((c) => (
              <button
                type="button"
                key={c.id}
                class={cat === c.name ? 'chip chip-active' : 'chip'}
                onClick={() => setCat(c.name)}
              >
                {categoryEmoji(c.name, categories)} {categoryLabel(c.name)}
              </button>
            ))}
          </div>
          {error && <div class="form-error">{error}</div>}
          <p class="cat-hint">Nowy produkt trafi na listę jedzenia i od razu do składu. Ikonkę, notatkę i cel ustawisz później w jego szczegółach.</p>
        </div>
      )}

      <div class="meal-picker-list">
        {matches.length === 0 && (
          <p class="muted">
            {onCreateFood ? 'Brak produktów — dodaj go powyżej „＋ Nowy produkt”.' : 'Brak produktów — dodaj je najpierw w zakładce Jedzenie.'}
          </p>
        )}
        {matches.map((f) => {
          const on = selected.includes(f.id);
          return (
            <button type="button" key={f.id} class={on ? 'meal-pick-row active' : 'meal-pick-row'} onClick={() => toggle(f.id)}>
              <span class="meal-pick-emoji">{foodIcon(f, categories)}</span>
              <span class="meal-pick-name">{f.name}</span>
              <span class="meal-pick-check">{on ? '✓' : '＋'}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
