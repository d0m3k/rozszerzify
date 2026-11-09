import { useEffect, useRef, useState } from 'preact/hooks';
import {
  Category,
  Food,
  Meal,
  categoryEmoji,
  categoryLabel,
  foodCategories,
  foodIcon,
  mealIcon,
  mealItemIcon,
  normalize,
  subMealIcon,
} from '../api';

interface Props {
  foods: Food[];
  meals: Meal[];
  categories: Category[];
  onBack: () => void;
  onPick: (food: Food) => void;
  onPlusMeal: (meal: Meal) => void;
  onOpenMeal: (id: number) => void;
  onQuickAdd: (name: string, categories: string[]) => void;
  onFullForm: () => void;
  /** Opens the "new meal" form with the typed name already filled in. */
  onNewMeal: (name: string) => void;
}

// One meal hit: tapping the row logs a try (+1), the chevron opens the meal.
function MealRow({
  m,
  cats,
  onPlus,
  onOpen,
}: {
  m: Meal;
  cats: Category[];
  onPlus: (m: Meal) => void;
  onOpen: (id: number) => void;
}) {
  const parts = [
    ...m.items.map((it) => `${mealItemIcon(it, cats)} ${it.name}`),
    ...m.sub_meals.map((sm) => `${subMealIcon(sm)} ${sm.name}`),
  ];
  const shown = parts.slice(0, 4).join(' · ');
  const more = parts.length > 4 ? ` · +${parts.length - 4}` : '';
  return (
    <div class="search-row" onClick={() => onPlus(m)}>
      <span class="search-emoji">{mealIcon(m)}</span>
      <div class="search-info">
        <span class="search-name">{m.name}</span>
        <span class="search-cat">
          {shown}
          {more}
        </span>
      </div>
      <span class="search-tries">{m.tries >= m.target ? '🎉' : `${m.tries}/${m.target}`}</span>
      <button
        class="search-open"
        aria-label={`otwórz danie ${m.name}`}
        onClick={(e) => {
          e.stopPropagation();
          onOpen(m.id);
        }}
      >
        ›
      </button>
    </div>
  );
}

export function SearchPage({
  foods,
  meals,
  categories,
  onBack,
  onPick,
  onPlusMeal,
  onOpenMeal,
  onQuickAdd,
  onFullForm,
  onNewMeal,
}: Props) {
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

  // Meals match by their own name or by an ingredient — "marchewka" also finds
  // "kasza z marchewką", so a whole dish can be logged from here.
  const mealMatches = query
    ? meals
        .filter((m) => normalize(m.name).includes(nq) || m.items.some((it) => normalize(it.name).includes(nq)))
        .sort((a, b) => {
          const sa = normalize(a.name).startsWith(nq) ? 0 : 1;
          const sb = normalize(b.name).startsWith(nq) ? 0 : 1;
          return sa - sb || a.name.localeCompare(b.name, 'pl');
        })
    : [];

  const exact = query !== '' && foods.some((f) => normalize(f.name) === nq);
  const exactMeal = query !== '' && meals.some((m) => normalize(m.name) === nq);
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
          placeholder="Co dziś zjadł — produkt albo danie?"
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
          {!exactMeal && (
            <button class="link-btn" onClick={() => onNewMeal(q.trim())}>
              🍲 Złóż z tego nowe danie →
            </button>
          )}
        </div>
      )}

      {!showAdd && (
        <button class="link-btn search-new-food" onClick={onFullForm}>
          ＋ Nowy produkt — bez zapisywania próby
        </button>
      )}

      {!query && (
        <p class="search-hint muted">
          Szybkie logowanie: wpisz lub wybierz produkt bądź całe danie, potem tylko ocena. 🍼
          <br />
          Polskie znaki niepotrzebne — „wolowina” znajdzie „wołowinę”.
        </p>
      )}

      {!query && meals.length > 0 && (
        <>
          <h3 class="search-group-title">🍲 Dania</h3>
          <div class="search-results">
            {meals.map((m) => (
              <MealRow key={m.id} m={m} cats={categories} onPlus={onPlusMeal} onOpen={onOpenMeal} />
            ))}
          </div>
          <button class="link-btn search-new-food" onClick={() => onNewMeal('')}>
            ＋ Złóż nowe danie
          </button>
        </>
      )}

      {query && mealMatches.length > 0 && (
        <>
          <h3 class="search-group-title">🍲 Dania</h3>
          <div class="search-results">
            {mealMatches.map((m) => (
              <MealRow key={m.id} m={m} cats={categories} onPlus={onPlusMeal} onOpen={onOpenMeal} />
            ))}
          </div>
        </>
      )}

      {query && matches.length > 0 && (
        <>
          <h3 class="search-group-title">🍽️ Produkty</h3>
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
                  {f.meal_names.length > 0 && <span class="search-meals">🍲 w daniach: {f.meal_names.join(' · ')}</span>}
                </div>
                <span class="search-tries">
                  {f.tries >= f.target ? '🎉' : `${f.tries}/${f.target}`}
                </span>
              </div>
            ))}
          </div>
        </>
      )}

      {query && matches.length === 0 && mealMatches.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍴</div>
          <p>Nic nie znaleziono{showAdd ? ' — dodaj wyżej!' : '.'}</p>
        </div>
      )}
    </div>
  );
}
