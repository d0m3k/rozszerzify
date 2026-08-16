import { useState } from 'preact/hooks';
import { RATINGS } from '../api';
import { NotePresets } from './NotePresets';

interface Props {
  foodName: string;
  onSelect: (rating: number, note: string) => void;
  onClose: () => void;
}

export function RatingSheet({ foodName, onSelect, onClose }: Props) {
  const [note, setNote] = useState('');

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
        <div class="rating-grid">
          {RATINGS.map((r) => (
            <button key={r.v} class="rating-option" onClick={() => onSelect(r.v, note.trim())}>
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