import { useState } from 'preact/hooks';
import { Category, Food, foodCategories, foodIcon, normalize } from '../api';

interface Props {
  foods: Food[];
  categories: Category[];
  selected: number[];
  onChange: (ids: number[]) => void;
}

// Multi-select list of existing foods used to build a meal. Selected foods
// are listed first so the meal composition is always visible.
export function MealFoodPicker({ foods, categories, selected, onChange }: Props) {
  const [q, setQ] = useState('');
  const query = normalize(q.trim());

  function toggle(id: number) {
    onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);
  }

  const matches = foods
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
    .map((id) => foods.find((f) => f.id === id))
    .filter((f): f is Food => !!f);

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
      />
      <div class="meal-picker-list">
        {matches.length === 0 && <p class="muted">Brak produktów — dodaj je najpierw w zakładce Jedzenie.</p>}
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
