import { useState } from 'preact/hooks';
import {
  Category,
  Food,
  Stats,
  STATUS_META,
  SortMode,
  categoryEmoji,
  categoryLabel,
  foodCategories,
  foodIcon,
  foodStatus,
  progressPct,
  ratingMeta,
  sortFoods,
} from '../api';
import { EmojiField } from '../components/EmojiField';

interface Props {
  foods: Food[];
  stats: Stats | null;
  categories: Category[];
  onOpenDetail: (id: number) => void;
  onPlus: (food: Food) => void;
  onRenameCategory: (id: number, name: string, emoji: string) => Promise<void>;
  onDeleteCategory: (id: number) => Promise<void>;
}

const SORT_KEY = 'rz_sort';
const COLLAPSED_KEY = 'rz_collapsed_cats';

function FoodRow({ f, cats, showCat, onOpen, onPlus }: { f: Food; cats: Category[]; showCat: boolean; onOpen: (id: number) => void; onPlus: (f: Food) => void }) {
  const st = foodStatus(f);
  const meta = STATUS_META[st];
  const fill = st === 'ok' ? 'progress-fill done' : st === 'revisit' ? 'progress-fill revisit' : 'progress-fill';

  return (
    <div class="food-row" onClick={() => onOpen(f.id)}>
      <div class="food-info">
        <div class="food-line">
          <span class="food-icon" aria-hidden="true">{foodIcon(f, cats)}</span>
          <span class="food-name">{f.name}</span>
          {f.last_rating > 0 && f.tries > 0 && (
            <span class="food-last-emoji" title="ostatnia próba">{ratingMeta(f.last_rating).emoji}</span>
          )}
          <span class={`status-badge ${meta.cls}`}>{meta.label}</span>
        </div>
        {showCat && (
          <div class="food-cat">
            {foodCategories(f).map((c) => `${categoryEmoji(c, cats)} ${categoryLabel(c)}`).join(' · ')}
          </div>
        )}
        {f.recent_ratings.length > 0 && (
          <div class="minki" aria-label="ostatnie oceny (od najstarszej)">
            {[...f.recent_ratings].reverse().slice(0, 5).map((r, i) => (
              <span class="minka" key={i}>{ratingMeta(r).emoji}</span>
            ))}
          </div>
        )}
        <div class="food-progress">
          <div class="progress-track">
            <div className={fill} style={{ width: `${progressPct(f.tries, f.target)}%` }} />
          </div>
          <span class="progress-label">
            {f.tries >= f.target ? '🎉 cel' : `${f.tries}/${f.target} prób`}
          </span>
        </div>
        {f.last_note && <div class="food-last-note">📝 {f.last_note}</div>}
      </div>
      <div class="food-actions">
        <button
          class="btn-plus"
          aria-label="dodaj próbę"
          onClick={(e) => {
            e.stopPropagation();
            onPlus(f);
          }}
        >
          <span class="btn-plus-plus">+1</span>
          <span class="btn-plus-sub">spróbował</span>
        </button>
      </div>
    </div>
  );
}

export function FoodsPage({ foods, stats, categories, onOpenDetail, onPlus, onRenameCategory, onDeleteCategory }: Props) {
  const [sort, setSort] = useState<SortMode>(() => (localStorage.getItem(SORT_KEY) as SortMode) || 'priorytet');
  // Categories the parent folded away in the "Kategorie" view (persisted, so
  // the list keeps the shape they left it in).
  const [collapsed, setCollapsed] = useState<string[]>(() => {
    try {
      const raw = JSON.parse(localStorage.getItem(COLLAPSED_KEY) || '[]');
      return Array.isArray(raw) ? raw.filter((c) => typeof c === 'string') : [];
    } catch {
      return [];
    }
  });
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [emojiDraft, setEmojiDraft] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [catError, setCatError] = useState('');

  function setCollapsedCats(next: string[]) {
    setCollapsed(next);
    localStorage.setItem(COLLAPSED_KEY, JSON.stringify(next));
  }

  function toggleCat(cat: string) {
    setCollapsedCats(collapsed.includes(cat) ? collapsed.filter((c) => c !== cat) : [...collapsed, cat]);
  }

  function changeSort(s: SortMode) {
    setSort(s);
    localStorage.setItem(SORT_KEY, s);
  }

  function openEditor(cat: string) {
    setEditing(cat);
    setDraft(cat);
    setEmojiDraft(categories.find((c) => c.name === cat)?.emoji ?? '');
    setConfirmDelete(false);
    setCatError('');
  }

  async function saveRename(e: Event, id: number) {
    e.preventDefault();
    const name = draft.trim();
    if (!name) return;
    try {
      await onRenameCategory(id, name, emojiDraft);
      setEditing(null);
    } catch (err: unknown) {
      setCatError(err instanceof Error ? err.message : 'Nie udało się zmienić nazwy');
    }
  }

  async function removeCategory(id: number) {
    try {
      await onDeleteCategory(id);
      setEditing(null);
    } catch (err: unknown) {
      setCatError(err instanceof Error ? err.message : 'Nie udało się usunąć kategorii');
    }
  }

  const { groups, list } = sortFoods(foods, sort, categories.map((c) => c.name));
  const okCount = foods.filter((f) => foodStatus(f) === 'ok').length;
  const foldedCount = groups ? groups.filter(([cat]) => collapsed.includes(cat)).length : 0;

  return (
    <div class="page">
      <div class="stats-row">
        {stats && (
          <>
            {stats.birth_date && (
              <div class="stat-chip">
                <b>{stats.baby_age_months} mies. {stats.baby_age_days > 0 ? `${stats.baby_age_days} dn.` : ''}</b>
                <span>wiek malucha</span>
              </div>
            )}
            {stats.started ? (
              <div class="stat-chip">
                <b>{stats.days_since_start}</b>
                <span>dzień od startu</span>
              </div>
            ) : stats.days_until_start > 0 ? (
              <div class="stat-chip stat-chip-hot">
                <b>⏳ {stats.days_until_start}</b>
                <span>dn. do startu diety</span>
              </div>
            ) : null}
            <div class="stat-chip">
              <b>{stats.foods_total}</b>
              <span>produktów</span>
            </div>
            <div class="stat-chip stat-chip-ok">
              <b>✓ {okCount || stats.foods_ok}</b>
              <span>OK — nie trzeba już testować</span>
            </div>
            {stats.foods_tried_today > 0 && (
              <div class="stat-chip stat-chip-hot">
                <b>{stats.foods_tried_today}</b>
                <span>dziś</span>
              </div>
            )}
          </>
        )}
      </div>

      <div class="sort-bar">
        <button class={sort === 'priorytet' ? 'sort-btn active' : 'sort-btn'} onClick={() => changeSort('priorytet')}>
          🎯 Priorytet
        </button>
        <button class={sort === 'kategorie' ? 'sort-btn active' : 'sort-btn'} onClick={() => changeSort('kategorie')}>
          🗂️ Kategorie
        </button>
        <button class={sort === 'az' ? 'sort-btn active' : 'sort-btn'} onClick={() => changeSort('az')}>
          🔤 A–Z
        </button>
      </div>

      {sort === 'priorytet' && (
        <div class="sort-legend">
          Najpierw te, których warto spróbować: <b>nowe</b>, potem <b>wrócić</b> (nie zjadł),
          na końcu <b>✓ OK</b>.
        </div>
      )}

      {foods.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍽️</div>
          <p>Lista jest pusta — dodaj pierwsze jedzenie!</p>
        </div>
      )}

      {list && (
        <div class="food-list">
          {list.map((f) => (
            <FoodRow key={f.id} f={f} cats={categories} showCat={true} onOpen={onOpenDetail} onPlus={onPlus} />
          ))}
        </div>
      )}

      {groups && (
        <div class="groups-bar">
          <button
            class="group-toggle-all"
            onClick={() => setCollapsedCats(foldedCount > 0 ? [] : groups.map(([cat]) => cat))}
          >
            {foldedCount > 0 ? '↕ Rozwiń wszystko' : '↕ Zwiń wszystko'}
          </button>
        </div>
      )}

      {groups &&
        groups.map(([cat, items]) => {
          const folded = collapsed.includes(cat);
          const meta = categories.find((c) => c.name === cat);
          const isEditing = editing === cat;
          return (
            <section class="food-group" key={cat}>
              <div class="group-head">
                <button
                  class="group-title"
                  aria-expanded={!folded}
                  title={folded ? 'Rozwiń kategorię' : 'Zwiń kategorię'}
                  onClick={() => toggleCat(cat)}
                >
                  <span class="group-chevron">{folded ? '▶' : '▼'}</span>
                  {categoryEmoji(cat, categories)} {categoryLabel(cat)}
                  <span class="group-count">
                    {items.filter((f) => foodStatus(f) === 'ok').length}/{items.length}
                  </span>
                </button>
                {meta && (
                  <button
                    class="group-edit"
                    aria-label="edytuj kategorię"
                    onClick={() => (isEditing ? setEditing(null) : openEditor(cat))}
                  >
                    {isEditing ? '×' : '✏️'}
                  </button>
                )}
              </div>
              {isEditing && meta && (
                <div class="cat-edit">
                  <form class="cat-edit-row" onSubmit={(e) => saveRename(e, meta.id)}>
                    <span class="cat-edit-emoji" aria-hidden="true">{emojiDraft || '✨'}</span>
                    <input
                      class="input"
                      value={draft}
                      onInput={(e) => setDraft((e.target as HTMLInputElement).value)}
                      aria-label="nazwa kategorii"
                    />
                    <button class="btn-primary" type="submit" disabled={!draft.trim() || (draft.trim() === cat && emojiDraft === meta.emoji)}>
                      Zapisz
                    </button>
                  </form>
                  <div class="field">
                    <span class="field-label">Ikonka kategorii</span>
                    <EmojiField value={emojiDraft} onChange={setEmojiDraft} label="Ikonka kategorii" />
                  </div>
                  {confirmDelete ? (
                    <div class="cat-edit-row">
                      <span class="cat-edit-warn">Usunąć „{cat}”? Produkty zostaną w innych kategoriach.</span>
                      <button class="btn-danger" onClick={() => removeCategory(meta.id)}>
                        Tak, usuń
                      </button>
                      <button class="btn-ghost" onClick={() => setConfirmDelete(false)}>
                        Nie
                      </button>
                    </div>
                  ) : (
                    <button class="link-btn cat-edit-del" onClick={() => setConfirmDelete(true)}>
                      Usuń kategorię
                    </button>
                  )}
                  {catError && <div class="form-error">{catError}</div>}
                </div>
              )}
              {folded ? (
                <p class="group-collapsed-hint">zwinięte — tapnij, żeby rozwinąć</p>
              ) : (
                items.map((f) => (
                  <FoodRow key={f.id} f={f} cats={categories} showCat={false} onOpen={onOpenDetail} onPlus={onPlus} />
                ))
              )}
            </section>
          );
        })}
    </div>
  );
}