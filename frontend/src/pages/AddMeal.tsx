import { useEffect, useRef, useState } from 'preact/hooks';
import { Category, Food, Meal, api } from '../api';
import { EmojiField } from '../components/EmojiField';
import { MealFoodPicker } from '../components/MealFoodPicker';
import { MealSubMealPicker } from '../components/MealSubMealPicker';

interface Props {
  foods: Food[];
  meals: Meal[];
  categories: Category[];
  onBack: () => void;
  onAdded: (id: number) => void;
  /** Creates a missing product from inside the composition picker. */
  onCreateFood: (input: { name: string; category: string }) => Promise<Food>;
}

const DEFAULT_TARGET = 15;

export function AddMealPage({ foods, meals, categories, onBack, onAdded, onCreateFood }: Props) {
  const [name, setName] = useState('');
  const [emoji, setEmoji] = useState('');
  const [notes, setNotes] = useState('');
  const [target, setTarget] = useState(String(DEFAULT_TARGET));
  const [items, setItems] = useState<number[]>([]);
  const [subMeals, setSubMeals] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);

  const hasComposition = items.length + subMeals.length > 0;

  useEffect(() => {
    requestAnimationFrame(() => nameRef.current?.focus());
  }, []);

  async function submit(e: Event) {
    e.preventDefault();
    const n = name.trim();
    if (!n || !hasComposition) return;
    setBusy(true);
    setError('');
    try {
      const meal = await api.createMeal({
        name: n,
        emoji,
        notes,
        target: Math.max(1, parseInt(target) || DEFAULT_TARGET),
        items,
        sub_meals: subMeals,
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
          <MealFoodPicker foods={foods} categories={categories} selected={items} onChange={setItems} onCreateFood={onCreateFood} />
        </div>

        <div class="field">
          <span class="field-label">…a może z gotowych dań? (np. zupka na rosole)</span>
          <MealSubMealPicker meals={meals} mealId={0} selected={subMeals} onChange={setSubMeals} />
          <p class="cat-hint">
            Zjedzenie dania doda +1 daniu, każdemu produktowi <b>i każdemu daniu-składnikowi</b>. Produkt lub danie
            występujące kilka razy w składzie liczy się tylko raz.
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

        {!hasComposition && <p class="muted">Wybierz co najmniej jeden produkt lub danie.</p>}
        {error && <div class="form-error">{error}</div>}
        <button class="btn-primary btn-block btn-lg" type="submit" disabled={busy || !name.trim() || !hasComposition}>
          {busy ? 'Dodawanie…' : 'Dodaj danie'}
        </button>
      </form>
    </div>
  );
}
