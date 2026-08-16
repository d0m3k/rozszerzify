import { useState } from 'preact/hooks';
import {
  Food,
  Stats,
  STATUS_META,
  SortMode,
  categoryEmoji,
  categoryLabel,
  foodStatus,
  progressPct,
  ratingMeta,
  sortFoods,
} from '../api';

interface Props {
  foods: Food[];
  stats: Stats | null;
  onOpenDetail: (id: number) => void;
  onPlus: (food: Food) => void;
}

const SORT_KEY = 'rz_sort';

function FoodRow({ f, showCat, onOpen, onPlus }: { f: Food; showCat: boolean; onOpen: (id: number) => void; onPlus: (f: Food) => void }) {
  const st = foodStatus(f);
  const meta = STATUS_META[st];
  const fill = st === 'ok' ? 'progress-fill done' : st === 'revisit' ? 'progress-fill revisit' : 'progress-fill';

  return (
    <div class="food-row" onClick={() => onOpen(f.id)}>
      <div class="food-info">
        <div class="food-line">
          <span class="food-name">{f.name}</span>
          {f.last_rating > 0 && f.tries > 0 && (
            <span class="food-last-emoji" title="ostatnia próba">{ratingMeta(f.last_rating).emoji}</span>
          )}
          <span class={`status-badge ${meta.cls}`}>{meta.label}</span>
        </div>
        {showCat && <div class="food-cat">{categoryEmoji(f.category)} {categoryLabel(f.category)}</div>}
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

export function FoodsPage({ foods, stats, onOpenDetail, onPlus }: Props) {
  const [sort, setSort] = useState<SortMode>(() => (localStorage.getItem(SORT_KEY) as SortMode) || 'priorytet');

  function changeSort(s: SortMode) {
    setSort(s);
    localStorage.setItem(SORT_KEY, s);
  }

  const { groups, list } = sortFoods(foods, sort);
  const okCount = foods.filter((f) => foodStatus(f) === 'ok').length;

  return (
    <div class="page">
      <div class="stats-row">
        {stats && (
          <>
            {stats.birth_date && (
              <div class="stat-chip">
                <b>{stats.baby_age_months} mies. {stats.baby_age_days > 0 ? `${stats.baby_age_days} dn.` : ''}</b>
                <span>wiek Krzyśka</span>
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
            <FoodRow key={f.id} f={f} showCat={true} onOpen={onOpenDetail} onPlus={onPlus} />
          ))}
        </div>
      )}

      {groups &&
        groups.map(([cat, items]) => (
          <section class="food-group" key={cat}>
            <h3 class="group-title">
              {categoryEmoji(cat)} {categoryLabel(cat)}
              <span class="group-count">
                {items.filter((f) => foodStatus(f) === 'ok').length}/{items.length}
              </span>
            </h3>
            {items.map((f) => (
              <FoodRow key={f.id} f={f} showCat={false} onOpen={onOpenDetail} onPlus={onPlus} />
            ))}
          </section>
        ))}
    </div>
  );
}