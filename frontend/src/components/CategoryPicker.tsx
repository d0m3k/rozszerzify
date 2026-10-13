import { useState } from 'preact/hooks';
import { Category, categoryEmoji, categoryLabel } from '../api';

interface Props {
  categories: Category[];
  selected: string[];
  onChange: (next: string[]) => void;
  /** Creates the category server-side and selects it (throws on failure). */
  onCreate: (name: string) => Promise<void>;
}

// Multi-select category chips + inline "new category", shared by the add form
// and the food detail settings.
export function CategoryPicker({ categories, selected, onChange, onCreate }: Props) {
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  function toggle(name: string) {
    if (selected.includes(name)) {
      // A food always keeps at least one category (the first is the primary).
      if (selected.length === 1) return;
      onChange(selected.filter((c) => c !== name));
      return;
    }
    onChange([...selected, name]);
  }

  async function submitNew(e?: Event) {
    e?.preventDefault();
    const name = draft.trim();
    if (!name || busy) return;
    setBusy(true);
    setError('');
    try {
      await onCreate(name);
      setDraft('');
      setAdding(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Nie udało się dodać kategorii');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div class="cat-picker">
      <div class="chip-grid">
        {categories.map((c) => (
          <button
            type="button"
            key={c.id}
            class={selected.includes(c.name) ? 'chip chip-active' : 'chip'}
            onClick={() => toggle(c.name)}
          >
            {categoryEmoji(c.name, categories)} {categoryLabel(c.name)}
          </button>
        ))}
        {!adding && (
          <button type="button" class="chip chip-new" onClick={() => setAdding(true)}>
            ＋ Nowa kategoria
          </button>
        )}
      </div>

      {adding && (
        // Deliberately not a <form>: this picker lives inside the food form.
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
            placeholder="np. zupy, desery…"
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
      )}
      {error && <div class="form-error">{error}</div>}
      <p class="cat-hint">Produkt może być w kilku kategoriach (np. jajko: alergeny + nabiał).</p>
    </div>
  );
}
