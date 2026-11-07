import { useState } from 'preact/hooks';
import { Meal, forbiddenComponents, mealIcon, normalize, subMealIcon } from '../api';

interface Props {
  meals: Meal[];
  /** Id of the meal being edited (0 while creating a new one). */
  mealId: number;
  selected: number[];
  onChange: (ids: number[]) => void;
}

// Multi-select list of other meals used as components of this one. Meals that
// already contain this meal (and the meal itself) are hidden: the API rejects
// those as well, this just avoids a pointless round-trip.
export function MealSubMealPicker({ meals, mealId, selected, onChange }: Props) {
  const [q, setQ] = useState('');
  const query = normalize(q.trim());

  const forbidden = mealId > 0 ? forbiddenComponents(meals, mealId) : new Set<number>();
  const candidates = meals.filter((m) => !forbidden.has(m.id));
  const matches = candidates
    .filter((m) => !query || normalize(m.name).includes(query))
    .sort((a, b) => {
      const sa = normalize(a.name).startsWith(query) ? 0 : 1;
      const sb = normalize(b.name).startsWith(query) ? 0 : 1;
      return sa - sb || a.name.localeCompare(b.name, 'pl');
    });

  function toggle(id: number) {
    onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);
  }

  const selectedMeals = selected
    .map((id) => meals.find((m) => m.id === id))
    .filter((m): m is Meal => !!m);

  return (
    <div class="meal-picker">
      {selectedMeals.length > 0 && (
        <div class="meal-selected">
          {selectedMeals.map((m) => (
            <button type="button" key={m.id} class="chip chip-active meal-selected-chip" onClick={() => toggle(m.id)}>
              {mealIcon(m)} {m.name} <span class="meal-selected-x">×</span>
            </button>
          ))}
        </div>
      )}
      <input
        class="input"
        value={q}
        onInput={(e) => setQ((e.target as HTMLInputElement).value)}
        placeholder="Szukaj dania, które wchodzi w skład…"
        autocapitalize="none"
        autoComplete="off"
      />
      <div class="meal-picker-list">
        {candidates.length === 0 && (
          <p class="muted">Brak innych dań — dodaj najpierw danie, które chcesz tu użyć (np. rosół).</p>
        )}
        {candidates.length > 0 && matches.length === 0 && <p class="muted">Brak pasujących dań.</p>}
        {matches.map((m) => {
          const on = selected.includes(m.id);
          return (
            <button type="button" key={m.id} class={on ? 'meal-pick-row active' : 'meal-pick-row'} onClick={() => toggle(m.id)}>
              <span class="meal-pick-emoji">{mealIcon(m)}</span>
              <span class="meal-pick-name">{m.name}</span>
              <span class="meal-pick-meta">
                {m.expanded_foods} {m.expanded_foods === 1 ? 'produkt' : 'produktów'}
              </span>
              <span class="meal-pick-check">{on ? '✓' : '＋'}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
