import { RATINGS } from '../api';

interface Props {
  foodName: string;
  onSelect: (rating: number) => void;
  onClose: () => void;
}

export function RatingSheet({ foodName, onSelect, onClose }: Props) {
  return (
    <div class="sheet-overlay" onClick={onClose}>
      <div class="sheet" onClick={(e) => e.stopPropagation()}>
        <div class="sheet-handle" />
        <h3 class="sheet-title">
          Jak posmakowała <b>{foodName}</b>?
        </h3>
        <div class="rating-grid">
          {RATINGS.map((r) => (
            <button key={r.v} class="rating-option" onClick={() => onSelect(r.v)}>
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