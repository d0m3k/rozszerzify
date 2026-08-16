import { useEffect, useState } from 'preact/hooks';
import { Food, LogEntry, RATINGS, api, categoryLabel, fmtDateTime, foodEmoji, progressPct, ratingMeta } from '../api';

interface Props {
  food: Food;
  onBack: () => void;
  onPlus: () => void;
  onUntry: (entry?: LogEntry) => void;
  onRemoveLog: (entry: LogEntry) => void;
  onSave: (patch: { notes?: string; target?: number }) => Promise<void>;
  onDelete: () => void;
}

export function FoodDetailPage({ food, onBack, onPlus, onUntry, onRemoveLog, onSave, onDelete }: Props) {
  const [log, setLog] = useState<LogEntry[] | null>(null);
  const [notes, setNotes] = useState(food.notes);
  const [target, setTarget] = useState(String(food.target));
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [saved, setSaved] = useState(false);

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
      await onSave({ notes, target: Math.max(1, parseInt(target) || 15) });
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
    } finally {
      setBusy(false);
    }
  }

  // fetch log when food.id changes

  const maxRatingCount = Math.max(1, ...RATINGS.map((r) => log?.filter((l) => l.rating === r.v).length ?? 0));

  return (
    <div class="page">
      <div class="detail-header">
        <button class="btn-ghost btn-back" onClick={onBack}>
          ← Wróć
        </button>
        <div class="detail-title">
          <h2>
            {foodEmoji(food.name, food.category)} {food.name}
          </h2>
          <span class="detail-cat">{categoryLabel(food.category)}</span>
        </div>
      </div>

      <div class="detail-counter-card">
        <div class="detail-counter">
          <button class="counter-btn counter-minus" onClick={() => onUntry(log && log.length > 0 ? log[0] : undefined)} aria-label="cofnij ostatnią próbę">
            −
          </button>
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
            Jeszcze <b>{food.target - food.tries}</b> prób do celu. Małe zwycięstwa codziennie! 💪
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
            {log.map((l) => (
              <li class="log-entry" key={l.id}>
                <span class="log-emoji">{ratingMeta(l.rating).emoji}</span>
                <span class="log-body">
                  <span class="log-note">{l.note || ratingMeta(l.rating).label.toLowerCase()}</span>
                  <span class="log-date">{fmtDateTime(l.tried_at)}</span>
                </span>
                <button
                  class="log-del"
                  aria-label="usuń tę próbę"
                  onClick={() => onRemoveLog(l)}
                >
                  ✕
                </button>
              </li>
            ))}
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