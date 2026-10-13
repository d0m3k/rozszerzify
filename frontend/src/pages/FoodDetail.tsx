import { useEffect, useState } from 'preact/hooks';
import { Category, Food, LogEntry, RATINGS, STATUS_META, api, categoryEmoji, categoryLabel, fmtDateTime, foodCategories, foodEmoji, foodIcon, foodStatus, localInputValue, progressPct, ratingMeta } from '../api';
import { CategoryPicker } from '../components/CategoryPicker';
import { EmojiField } from '../components/EmojiField';

interface Props {
  food: Food;
  categories: Category[];
  onBack: () => void;
  onPlus: () => void;
  onRemoveLog: (entry: LogEntry) => void;
  onSave: (patch: { emoji?: string; categories?: string[]; notes?: string; target?: number }) => Promise<void>;
  onCreateCategory: (name: string) => Promise<Category>;
  onDelete: () => void;
}

export function FoodDetailPage({ food, categories, onBack, onPlus, onRemoveLog, onSave, onCreateCategory, onDelete }: Props) {
  const st = foodStatus(food);
  const [log, setLog] = useState<LogEntry[] | null>(null);
  const [cats, setCats] = useState(foodCategories(food));
  const [emoji, setEmoji] = useState(food.emoji);
  const [notes, setNotes] = useState(food.notes);
  const [target, setTarget] = useState(String(food.target));
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [saved, setSaved] = useState(false);
  // In-place editing of one history entry (rating / note / when it was eaten).
  const [editId, setEditId] = useState<number | null>(null);
  const [editRating, setEditRating] = useState(3);
  const [editNote, setEditNote] = useState('');
  const [editWhen, setEditWhen] = useState('');
  const [editError, setEditError] = useState('');
  const [logBusy, setLogBusy] = useState(false);

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
      await api.updateLog(food.id, l.id, {
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

  function loadLog() {
    api
      .foodLog(food.id)
      .then(setLog)
      .catch(() => setLog([]));
  }

  useEffect(() => {
    setLog(null);
    loadLog();
  }, [food.id, food.tries]);

  async function save() {
    setBusy(true);
    try {
      await onSave({ categories: cats, emoji, notes, target: Math.max(1, parseInt(target) || 15) });
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
    } finally {
      setBusy(false);
    }
  }

  // fetch log when food.id changes

  const maxRatingCount = Math.max(1, ...RATINGS.map((r) => log?.filter((l) => l.rating === r.v).length ?? 0));

  async function addCategory(name: string) {
    const created = await onCreateCategory(name);
    setCats((prev) => (prev.includes(created.name) ? prev : [...prev, created.name]));
  }

  return (
    <div class="page">
      <div class="detail-header">
        <button class="btn-ghost btn-back" onClick={onBack}>
          ← Wróć
        </button>
        <div class="detail-title">
          <h2>
            {foodIcon(food, categories)} {food.name}
            <span class={`status-badge ${STATUS_META[st].cls}`}>{STATUS_META[st].label}</span>
          </h2>
          <span class="detail-cat">
            {foodCategories(food).map((c) => `${categoryEmoji(c, categories)} ${categoryLabel(c)}`).join(' · ')}
          </span>
        </div>
      </div>

      <div class="detail-counter-card">
        <div class="detail-counter">
          <div class="counter-center">
            <div class="counter-number">{food.tries}</div>
            <div class="counter-label">
              {food.tries >= food.target ? 'prób — cel osiągnięty! 🎉' : `prób z ${food.target}`}
            </div>
          </div>
          <button class="counter-btn counter-plus" onClick={onPlus} aria-label="dodaj próbę">
            <span class="counter-plus-label">+1</span>
          </button>
        </div>
        <div class="progress-track progress-lg">
          <div
            class={food.tries >= food.target ? 'progress-fill done' : 'progress-fill'}
            style={{ width: `${progressPct(food.tries, food.target)}%` }}
          />
        </div>
        {food.tries < food.target && (
          <p class="detail-hint">
            {st === 'ok'
              ? 'Ostatnia próba zielona — uznajemy za OK, choć cel nie padł. Skup się na innych! 💚'
              : st === 'revisit'
              ? `Ostatnio nie zjadł — wróć do niego za jakiś czas. Jeszcze ${food.target - food.tries} prób do celu.`
              : st === 'new'
              ? 'Jeszcze nie próbował — pierwsza próba czeka! 🧪'
              : `Jeszcze ${food.target - food.tries} prób do celu. Małe zwycięstwa codziennie! 💪`}
          </p>
        )}
      </div>

      {food.rating_count > 0 && (
        <section class="card">
          <h3 class="card-title">
            Jak zjada — średnia {food.rating_avg.toFixed(1)}{' '}
            {ratingMeta(Math.round(food.rating_avg)).emoji}
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
        <h3 class="card-title">Ustawienia</h3>
        <div class="field">
          <span class="field-label">Ikonka produktu</span>
          <EmojiField value={emoji} onChange={setEmoji} label="Ikonka produktu" />
        </div>
        <div class="field">
          <span class="field-label">Kategorie</span>
          <CategoryPicker categories={categories} selected={cats} onChange={setCats} onCreate={addCategory} />
        </div>
        <label class="field">
          <span class="field-label">Cel (ile prób zanim uznamy, że nie smakuje)</span>
          <input class="input" type="number" min={1} max={100} value={target} onInput={(e) => setTarget((e.target as HTMLInputElement).value)} />
        </label>
        <label class="field">
          <span class="field-label">Notatki (np. jak podane, z czym)</span>
          <textarea class="input" rows={3} value={notes} onInput={(e) => setNotes((e.target as HTMLTextAreaElement).value)} placeholder="np. gotowana marchewka w słupkach…" />
        </label>
        <button class="btn-primary" onClick={save} disabled={busy}>
          {saved ? '✓ Zapisano' : busy ? 'Zapisywanie…' : 'Zapisz'}
        </button>
      </section>

      <section class="card">
        <h3 class="card-title">Historia prób</h3>
        {log === null && <p class="muted">Ładowanie…</p>}
        {log !== null && log.length === 0 && <p class="muted">Brak prób — naciśnij +1 po każdym podaniu.</p>}
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
                    <span class="log-date">{fmtDateTime(l.tried_at)}</span>
                  </span>
                  <button class="log-edit-btn" aria-label="edytuj tę próbę" onClick={() => startLogEdit(l)}>
                    ✏️
                  </button>
                  <button
                    class="log-del"
                    aria-label="usuń tę próbę"
                    onClick={() => onRemoveLog(l)}
                  >
                    ✕
                  </button>
                </li>
              )
            )}
          </ul>
        )}
      </section>

      <section class="card danger-zone">
        {!confirmDelete ? (
          <button class="btn-danger" onClick={() => setConfirmDelete(true)}>
            Usuń «{food.name}» z listy
          </button>
        ) : (
          <div class="confirm-delete">
            <p>Na pewno usunąć? Historia prób też zniknie.</p>
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