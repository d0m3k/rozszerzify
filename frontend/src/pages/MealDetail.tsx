import { useEffect, useState } from 'preact/hooks';
import {
  Category,
  Food,
  LogEntry,
  Meal,
  RATINGS,
  STATUS_META,
  api,
  fmtDateTime,
  localInputValue,
  mealIcon,
  mealItemIcon,
  mealStatus,
  progressPct,
  ratingMeta,
  subMealIcon,
} from '../api';
import { EmojiField } from '../components/EmojiField';
import { MealFoodPicker } from '../components/MealFoodPicker';
import { MealSubMealPicker } from '../components/MealSubMealPicker';

interface Props {
  meal: Meal;
  foods: Food[];
  meals: Meal[];
  categories: Category[];
  onBack: () => void;
  onPlus: () => void;
  onRemoveLog: (entry: LogEntry) => void;
  onSave: (patch: { name?: string; emoji?: string; notes?: string; target?: number; items?: number[]; sub_meals?: number[] }) => Promise<void>;
  onDelete: () => void;
  onOpenFood: (id: number) => void;
  onOpenMeal: (id: number) => void;
}

export function MealDetailPage({ meal, foods, meals, categories, onBack, onPlus, onRemoveLog, onSave, onDelete, onOpenFood, onOpenMeal }: Props) {
  const st = mealStatus(meal);
  const [log, setLog] = useState<LogEntry[] | null>(null);
  const [name, setName] = useState(meal.name);
  const [emoji, setEmoji] = useState(meal.emoji);
  const [notes, setNotes] = useState(meal.notes);
  const [target, setTarget] = useState(String(meal.target));
  const [items, setItems] = useState<number[]>(meal.items.map((it) => it.food_id));
  const [subMeals, setSubMeals] = useState<number[]>(meal.sub_meals.map((sm) => sm.meal_id));
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [editRating, setEditRating] = useState(3);
  const [editNote, setEditNote] = useState('');
  const [editWhen, setEditWhen] = useState('');
  const [editError, setEditError] = useState('');
  const [logBusy, setLogBusy] = useState(false);

  function loadLog() {
    api
      .mealLog(meal.id)
      .then(setLog)
      .catch(() => setLog([]));
  }

  useEffect(() => {
    setLog(null);
    loadLog();
  }, [meal.id, meal.tries]);

  // The meal can change underneath (a try, a refresh): keep the form in sync.
  useEffect(() => {
    setName(meal.name);
    setEmoji(meal.emoji);
    setNotes(meal.notes);
    setTarget(String(meal.target));
    setItems(meal.items.map((it) => it.food_id));
    setSubMeals(meal.sub_meals.map((sm) => sm.meal_id));
  }, [meal.id, meal.name, meal.emoji, meal.notes, meal.target, meal.items, meal.sub_meals]);

  function startLogEdit(l: LogEntry) {
    setEditId(l.id);
    setEditRating(l.rating);
    setEditNote(l.note);
    setEditWhen(localInputValue(new Date(l.tried_at)));
    setEditError('');
  }

  async function saveLogEdit(l: LogEntry) {
    setLogBusy(true);
    setEditError('');
    try {
      await api.updateMealLog(meal.id, l.id, {
        rating: editRating,
        note: editNote,
        tried_at: new Date(editWhen).toISOString(),
      });
      setEditId(null);
      loadLog();
    } catch (err: unknown) {
      setEditError(err instanceof Error ? err.message : 'Nie udało się zapisać próby');
    } finally {
      setLogBusy(false);
    }
  }

  async function save() {
    setBusy(true);
    try {
      await onSave({
        name: name.trim() || meal.name,
        emoji,
        notes,
        target: Math.max(1, parseInt(target) || 15),
        items,
        sub_meals: subMeals,
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
    } finally {
      setBusy(false);
    }
  }

  const maxRatingCount = Math.max(1, ...RATINGS.map((r) => log?.filter((l) => l.rating === r.v).length ?? 0));

  return (
    <div class="page">
      <div class="detail-header">
        <button class="btn-ghost btn-back" onClick={onBack}>
          ← Wróć
        </button>
        <div class="detail-title">
          <h2>
            {mealIcon(meal)} {meal.name}
            <span class={`status-badge ${STATUS_META[st].cls}`}>{STATUS_META[st].label}</span>
          </h2>
          <span class="detail-cat">
            🍲 danie · {meal.items.length} {meal.items.length === 1 ? 'produkt' : meal.items.length < 5 ? 'produkty' : 'produktów'}
            {meal.sub_meals.length > 0 && <> · {meal.sub_meals.length} {meal.sub_meals.length === 1 ? 'danie' : 'dania'} w składzie</>}
          </span>
        </div>
      </div>

      <div class="detail-counter-card">
        <div class="detail-counter">
          <div class="counter-center">
            <div class="counter-number">{meal.tries}</div>
            <div class="counter-label">
              {meal.tries >= meal.target ? 'prób — cel osiągnięty! 🎉' : `prób z ${meal.target}`}
            </div>
          </div>
          <button class="counter-btn counter-plus" onClick={onPlus} aria-label="dodaj próbę dania">
            <span class="counter-plus-label">+1</span>
          </button>
        </div>
        <div class="progress-track progress-lg">
          <div
            class={meal.tries >= meal.target ? 'progress-fill done' : 'progress-fill'}
            style={{ width: `${progressPct(meal.tries, meal.target)}%` }}
          />
        </div>
        <p class="detail-hint">
          +1 dla dania oznacza zjedzony posiłek: próba rośnie też u {meal.expanded_foods}{' '}
          {meal.expanded_foods === 1 ? 'produktu' : 'produktów'}
          {meal.expanded_meals > 0 && (
            <> i {meal.expanded_meals} {meal.expanded_meals === 1 ? 'dania-składnika' : 'dań-składników'}</>
          )}
          . Ten sam składnik liczony jest tylko raz.
        </p>
      </div>

      <section class="card">
        <h3 class="card-title">🍽️ Z czego składa się to danie</h3>
        <div class="meal-detail-items">
          {meal.items.map((it) => (
            <button class="meal-detail-item" key={it.food_id} onClick={() => onOpenFood(it.food_id)}>
              <span class="meal-detail-item-emoji">{mealItemIcon(it, categories)}</span>
              <span class="meal-detail-item-name">{it.name}</span>
              <span class="meal-detail-item-go">›</span>
            </button>
          ))}
        </div>

        {meal.sub_meals.length > 0 && (
          <>
            <h4 class="meal-sub-heading">Dania w składzie</h4>
            <div class="meal-detail-items">
              {meal.sub_meals.map((sm) => (
                <button class="meal-detail-item meal-detail-sub" key={sm.meal_id} onClick={() => onOpenMeal(sm.meal_id)}>
                  <span class="meal-detail-item-emoji">{subMealIcon(sm)}</span>
                  <span class="meal-detail-item-name">{sm.name}</span>
                  <span class="meal-sub-badge">danie</span>
                  <span class="meal-detail-item-go">›</span>
                </button>
              ))}
            </div>
          </>
        )}

        <p class="cat-hint">
          Każde zjedzenie dania zapisuje próbę każdemu produktowi oraz każdemu daniu-składnikowi (bez powtórzeń) —
          w ich historiach zobaczysz, że pochodzą z tego dania. Zmień skład poniżej i zapisz.
        </p>

        <span class="field-label">Produkty</span>
        <MealFoodPicker foods={foods} categories={categories} selected={items} onChange={setItems} />

        <span class="field-label meal-sub-label">Dania-składniki</span>
        <MealSubMealPicker meals={meals} mealId={meal.id} selected={subMeals} onChange={setSubMeals} />
      </section>

      {meal.rating_count > 0 && (
        <section class="card">
          <h3 class="card-title">
            Jak zjadane — średnia {meal.rating_avg.toFixed(1)} {ratingMeta(Math.round(meal.rating_avg)).emoji}
          </h3>
          <div class="rating-bars">
            {RATINGS.map((r) => {
              const n = log?.filter((l) => l.rating === r.v).length ?? 0;
              return (
                <div class="rating-bar-row" key={r.v}>
                  <span class="rating-bar-emoji">{r.emoji}</span>
                  <div class="rating-bar-track">
                    <div class="rating-bar-fill" style={{ width: `${(n / maxRatingCount) * 100}%` }} />
                  </div>
                  <span class="rating-bar-count">{n}</span>
                </div>
              );
            })}
          </div>
        </section>
      )}

      <section class="card">
        <h3 class="card-title">Ustawienia dania</h3>
        <label class="field">
          <span class="field-label">Nazwa</span>
          <input class="input" value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
        </label>
        <div class="field">
          <span class="field-label">Ikonka dania</span>
          <EmojiField value={emoji} onChange={setEmoji} label="Ikonka dania" />
        </div>
        <label class="field">
          <span class="field-label">Cel (ile prób zanim uznamy, że danie nie smakuje)</span>
          <input class="input" type="number" min={1} max={100} value={target} onInput={(e) => setTarget((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">
          <span class="field-label">Notatki</span>
          <textarea class="input" rows={3} value={notes} onInput={(e) => setNotes((e.target as HTMLTextAreaElement).value)} placeholder="np. podawać z łyżeczki, ciepłe…" />
        </label>
        {items.length + subMeals.length === 0 && <p class="muted">Danie musi mieć co najmniej jeden produkt lub danie.</p>}
        <button class="btn-primary" onClick={save} disabled={busy || items.length + subMeals.length === 0}>
          {saved ? '✓ Zapisano' : busy ? 'Zapisywanie…' : 'Zapisz'}
        </button>
      </section>

      <section class="card">
        <h3 class="card-title">Historia zjedzonych dań</h3>
        {log === null && <p class="muted">Ładowanie…</p>}
        {log !== null && log.length === 0 && <p class="muted">Brak prób — naciśnij +1 po każdym podaniu dania.</p>}
        {log !== null && log.length > 0 && (
          <ul class="log-list">
            {log.map((l) =>
              editId === l.id ? (
                <li class="log-entry log-entry-edit" key={l.id}>
                  <div class="rating-grid rating-grid-sm">
                    {RATINGS.map((r) => (
                      <button
                        key={r.v}
                        type="button"
                        class={editRating === r.v ? 'rating-option rating-option-sm active' : 'rating-option rating-option-sm'}
                        onClick={() => setEditRating(r.v)}
                        title={r.label}
                      >
                        <span class="rating-emoji">{r.emoji}</span>
                      </button>
                    ))}
                  </div>
                  <div class="log-edit-row">
                    <input
                      class="input"
                      value={editNote}
                      onInput={(e) => setEditNote((e.target as HTMLInputElement).value)}
                      placeholder="notatka"
                      aria-label="notatka próby"
                    />
                  </div>
                  <div class="log-edit-row">
                    <input
                      class="input"
                      type="datetime-local"
                      value={editWhen}
                      max={localInputValue(new Date())}
                      onInput={(e) => setEditWhen((e.target as HTMLInputElement).value)}
                      aria-label="kiedy było jedzone"
                    />
                  </div>
                  {editError && <div class="form-error">{editError}</div>}
                  <div class="log-edit-actions">
                    <button class="btn-primary" onClick={() => saveLogEdit(l)} disabled={logBusy}>
                      {logBusy ? 'Zapisywanie…' : 'Zapisz'}
                    </button>
                    <button class="btn-ghost" onClick={() => setEditId(null)}>
                      Anuluj
                    </button>
                  </div>
                </li>
              ) : (
                <li class="log-entry" key={l.id}>
                  <span class="log-emoji">{ratingMeta(l.rating).emoji}</span>
                  <span class="log-body">
                    <span class="log-note">{l.note || ratingMeta(l.rating).label.toLowerCase()}</span>
                    <span class="log-date">
                      {fmtDateTime(l.tried_at)} ·{' '}
                      {l.parent_meal ? `🍲 część dania „${l.parent_meal}”` : '🍲 całe danie +1'}
                    </span>
                  </span>
                  <button class="log-edit-btn" aria-label="edytuj tę próbę" onClick={() => startLogEdit(l)}>
                    ✏️
                  </button>
                  {!l.parent_meal && (
                    <button class="log-del" aria-label="usuń tę próbę" onClick={() => onRemoveLog(l)}>
                      ✕
                    </button>
                  )}
                </li>
              )
            )}
          </ul>
        )}
      </section>

      <section class="card danger-zone">
        {!confirmDelete ? (
          <button class="btn-danger" onClick={() => setConfirmDelete(true)}>
            Usuń danie «{meal.name}»
          </button>
        ) : (
          <div class="confirm-delete">
            <p>Na pewno usunąć danie? Produkty i ich historia prób zostaną na liście.</p>
            <div class="confirm-actions">
              <button class="btn-danger" onClick={onDelete}>
                Tak, usuń
              </button>
              <button class="btn-ghost" onClick={() => setConfirmDelete(false)}>
                Anuluj
              </button>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
