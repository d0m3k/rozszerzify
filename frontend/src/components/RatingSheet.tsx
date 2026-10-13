import { useState } from 'preact/hooks';
import { RATINGS, localInputValue } from '../api';
import { NotePresets } from './NotePresets';

interface Props {
  foodName: string;
  /** triedAt is an RFC3339 timestamp, or undefined for "just now". */
  onSelect: (rating: number, note: string, triedAt?: string) => void;
  onClose: () => void;
}

export function RatingSheet({ foodName, onSelect, onClose }: Props) {
  const [note, setNote] = useState('');
  const [when, setWhen] = useState(() => localInputValue(new Date()));
  const [editWhen, setEditWhen] = useState(false);

  const maxNow = localInputValue(new Date());
  const isNow = when === maxNow;
  const whenDate = new Date(when); // no offset → parsed as local time
  const whenLabel = Number.isNaN(whenDate.getTime())
    ? when
    : whenDate.toLocaleString('pl-PL', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });

  return (
    <div class="sheet-overlay" onClick={onClose}>
      <div class="sheet" onClick={(e) => e.stopPropagation()}>
        <div class="sheet-handle" />
        <h3 class="sheet-title">
          Jak zjadł <b>{foodName}</b>?
        </h3>
        <input
          class="input sheet-note"
          value={note}
          onInput={(e) => setNote((e.target as HTMLInputElement).value)}
          placeholder="Notatka: jak podane, z czym…"
        />
        <NotePresets value={note} onChange={setNote} />
        <div class="sheet-when">
          <button
            type="button"
            class={isNow ? 'when-chip' : 'when-chip when-chip-set'}
            onClick={() => setEditWhen((v) => !v)}
          >
            🕒 {isNow ? 'Teraz' : whenLabel} <span class="when-chip-edit">{editWhen ? 'ukryj' : 'zmień'}</span>
          </button>
          {!isNow && (
            <button type="button" class="when-reset" onClick={() => { setWhen(maxNow); setEditWhen(false); }}>
              teraz
            </button>
          )}
          {editWhen && (
            <input
              class="input when-input"
              type="datetime-local"
              value={when}
              max={maxNow}
              onClick={(e) => e.stopPropagation()}
              onInput={(e) => setWhen((e.target as HTMLInputElement).value)}
            />
          )}
        </div>
        <div class="rating-grid">
          {RATINGS.map((r) => (
            <button
              key={r.v}
              class="rating-option"
              onClick={() => {
                // Untouched picker → no tried_at, so the server stamps NOW() and
                // a sheet left open for a few minutes still records the tap time.
                const ts = Number.isNaN(whenDate.getTime()) ? undefined : whenDate.toISOString();
                onSelect(r.v, note.trim(), isNow ? undefined : ts);
              }}
            >
              <span class="rating-emoji">{r.emoji}</span>
              <span class="rating-label">{r.label}</span>
            </button>
          ))}
        </div>
        <button class="btn-ghost sheet-cancel" onClick={onClose}>
          Anuluj
        </button>
      </div>
    </div>
  );
}
