import { useEffect, useRef, useState } from 'preact/hooks';
import { Category, Food, api } from '../api';
import { EmojiField } from '../components/EmojiField';
import { MealFoodPicker } from '../components/MealFoodPicker';

interface Props {
  foods: Food[];
  categories: Category[];
  onBack: () => void;
  onAdded: (id: number) => void;
}

const DEFAULT_TARGET = 15;

export function AddMealPage({ foods, categories, onBack, onAdded }: Props) {
  const [name, setName] = useState('');
  const [emoji, setEmoji] = useState('');
  const [notes, setNotes] = useState('');
  const [target, setTarget] = useState(String(DEFAULT_TARGET));
  const [items, setItems] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    requestAnimationFrame(() => nameRef.current?.focus());
  }, []);

  async function submit(e: Event) {
    e.preventDefault();
    const n = name.trim();
    if (!n || items.length === 0) return;
    setBusy(true);
    setError('');
    try {
      const meal = await api.createMeal({
        name: n,
        emoji,
        notes,
        target: Math.max(1, parseInt(target) || DEFAULT_TARGET),
        items,
      });
      onAdded(meal.id);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Nie udało się dodać dania');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div class="page">
      <div class="detail-header">
        <button class="btn-ghost btn-back" onClick={onBack}>
          ← Wróć
        </button>
        <div class="detail-title">
          <h2>🍲 Nowe danie</h2>
        </div>
      </div>

      <form class="card" onSubmit={submit}>
        <label class="field">
          <span class="field-label">Nazwa dania</span>
          <input
            class="input input-xl"
            value={name}
            onInput={(e) => setName((e.target as HTMLInputElement).value)}
            placeholder="np. kasza z marchewką"
            ref={nameRef}
            autocapitalize="none"
            autoComplete="off"
            autoFocus
            required
          />
        </label>

        <div class="field">
          <span class="field-label">Ikonka (opcjonalnie — domyślnie 🍲)</span>
          <EmojiField value={emoji} onChange={setEmoji} label="Ikonka dania" />
        </div>

        <div class="field">
          <span class="field-label">Z czego składa się danie? (wybierz produkty)</span>
          <MealFoodPicker foods={foods} categories={categories} selected={items} onChange={setItems} />
          <p class="cat-hint">
            Zjedzenie dania doda +1 próbę daniu i każdemu wybranemu produktowi. Produkt może należeć do wielu dań.
          </p>
        </div>

        <label class="field">
          <span class="field-label">Cel — po ilu próbach uznamy, że danie nie smakuje?</span>
          <input class="input" type="number" min={1} max={100} value={target} onInput={(e) => setTarget((e.target as HTMLInputElement).value)} />
        </label>

        <label class="field">
          <span class="field-label">Notatka (opcjonalnie)</span>
          <textarea class="input" rows={2} value={notes} onInput={(e) => setNotes((e.target as HTMLTextAreaElement).value)} placeholder="np. podawać letnie, z łyżeczki…" />
        </label>

        {items.length === 0 && <p class="muted">Wybierz co najmniej jeden produkt.</p>}
        {error && <div class="form-error">{error}</div>}
        <button class="btn-primary btn-block btn-lg" type="submit" disabled={busy || !name.trim() || items.length === 0}>
          {busy ? 'Dodawanie…' : 'Dodaj danie'}
        </button>
      </form>
    </div>
  );
}
