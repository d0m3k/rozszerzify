import { EMOJI_PALETTE } from '../api';

interface Props {
  value: string;
  onChange: (v: string) => void;
  /** Accessible label / placeholder for the free-text field. */
  label?: string;
}

// Compact icon picker: type/paste any emoji or tick one from the palette.
export function EmojiField({ value, onChange, label = 'Ikonka' }: Props) {
  return (
    <div class="emoji-field">
      <div class="emoji-row">
        <input
          class="input emoji-input"
          value={value}
          maxLength={4}
          onInput={(e) => onChange((e.target as HTMLInputElement).value)}
          placeholder="🙂"
          aria-label={label}
          autocapitalize="none"
          autoComplete="off"
        />
        <span class="emoji-preview" aria-hidden="true">{value || '—'}</span>
        {value && (
          <button type="button" class="btn-ghost emoji-clear" onClick={() => onChange('')}>
            wyczyść
          </button>
        )}
      </div>
      <div class="emoji-palette">
        {EMOJI_PALETTE.map((e) => (
          <button
            type="button"
            key={e}
            class={value === e ? 'emoji-chip active' : 'emoji-chip'}
            onClick={() => onChange(value === e ? '' : e)}
            aria-label={e}
          >
            {e}
          </button>
        ))}
      </div>
    </div>
  );
}
