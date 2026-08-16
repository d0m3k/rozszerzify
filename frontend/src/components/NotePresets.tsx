// Quick note presets — one tap fills/cycles the note in the rating sheet
// and the add-food form. Keeping the list short and practical.
export const NOTE_PRESETS = [
  'papka',
  'całe',
  'w kawałkach',
  'z kaszą',
  'na obiad',
  'na śniadanie',
];

interface Props {
  value: string;
  onChange: (v: string) => void;
}

export function NotePresets({ value, onChange }: Props) {
  return (
    <div class="note-presets">
      {NOTE_PRESETS.filter((p) => p !== value).map((p) => (
        <button key={p} type="button" class="chip chip-sm" onClick={() => onChange(p)}>
          {p}
        </button>
      ))}
    </div>
  );
}