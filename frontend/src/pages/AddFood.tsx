import { useEffect, useRef, useState } from 'preact/hooks';
import { Category, api } from '../api';
import { NotePresets } from '../components/NotePresets';
import { CategoryPicker } from '../components/CategoryPicker';

interface Props {
  categories: Category[];
  onBack: () => void;
  onAdded: () => void;
  onCreateCategory: (name: string) => Promise<Category>;
}

const DEFAULT_TARGET = 15;

export function AddFoodPage({ categories, onBack, onAdded, onCreateCategory }: Props) {
  const [name, setName] = useState('');
  const [cats, setCats] = useState<string[]>(['owoce']);
  const [target, setTarget] = useState(String(DEFAULT_TARGET));
  const [notes, setNotes] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);

  // Reliable way to pop the keyboard on mobile: focus right after mount.
  useEffect(() => {
    requestAnimationFrame(() => nameRef.current?.focus());
  }, []);

  async function submit(e: Event) {
    e.preventDefault();
    const n = name.trim();
    if (!n) return;
    setBusy(true);
    setError('');
    try {
      await api.createFood({
        name: n,
        categories: cats,
        target: Math.max(1, parseInt(target) || DEFAULT_TARGET),
        notes,
      });
      onAdded();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Nie udało się dodać');
    } finally {
      setBusy(false);
    }
  }

  async function addCategory(name: string) {
    const created = await onCreateCategory(name);
    setCats((prev) => [...prev, created.name]);
  }

  return (
    <div class="page">
      <div class="detail-header">
        <button class="btn-ghost btn-back" onClick={onBack}>
          ← Wróć
        </button>
        <div class="detail-title">
          <h2>➕ Nowe jedzenie</h2>
        </div>
      </div>

      <form class="card" onSubmit={submit}>
        <label class="field">
          <span class="field-label">Co to za jedzenie?</span>
          <input
            class="input input-xl"
            value={name}
            onInput={(e) => setName((e.target as HTMLInputElement).value)}
            placeholder="np. kalarepa, amarantus…"
            ref={nameRef}
            autocapitalize="none"
            autoComplete="off"
            autoFocus
            required
          />
        </label>

        <div class="field">
          <span class="field-label">Kategorie</span>
          <CategoryPicker categories={categories} selected={cats} onChange={setCats} onCreate={addCategory} />
        </div>

        <label class="field">
          <span class="field-label">Cel — po ilu próbach uznamy, że mu nie smakuje?</span>
          <input class="input" type="number" min={1} max={100} value={target} onInput={(e) => setTarget((e.target as HTMLInputElement).value)} />
        </label>

        <label class="field">
          <span class="field-label">Notatka (opcjonalnie)</span>
          <textarea class="input" rows={2} value={notes} onInput={(e) => setNotes((e.target as HTMLTextAreaElement).value)} placeholder="np. podawać z kaszą jaglaną…" />
          <NotePresets value={notes} onChange={setNotes} />
        </label>

        {error && <div class="form-error">{error}</div>}
        <button class="btn-primary btn-block btn-lg" type="submit" disabled={busy || !name.trim()}>
          {busy ? 'Dodawanie…' : 'Dodaj do listy'}
        </button>
      </form>
    </div>
  );
}