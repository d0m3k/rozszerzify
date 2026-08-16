import { Food, Stats, categoryEmoji, categoryLabel, progressPct, ratingMeta, sortCategories } from '../api';

interface Props {
  foods: Food[];
  stats: Stats | null;
  onOpenDetail: (id: number) => void;
  onPlus: (food: Food) => void;
  onMinus: (food: Food) => void;
}

export function FoodsPage({ foods, stats, onOpenDetail, onPlus, onMinus }: Props) {
  const cats = sortCategories([...new Set(foods.map((f) => f.category))]);
  const byCat: Record<string, Food[]> = {};
  for (const f of foods) {
    (byCat[f.category] ||= []).push(f);
  }
  for (const list of Object.values(byCat)) {
    list.sort((a, b) => a.name.localeCompare(b.name, 'pl'));
  }

  return (
    <div class="page">
      <div class="stats-row">
        {stats && (
          <>
            <div class="stat-chip">
              <b>{stats.days_since_start}</b>
              <span>dzień od startu</span>
            </div>
            <div class="stat-chip">
              <b>{stats.foods_total}</b>
              <span>produktów</span>
            </div>
            <div class="stat-chip">
              <b>{stats.foods_at_target}</b>
              <span>przetestowanych 🎉</span>
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

      {foods.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍽️</div>
          <p>Lista jest pusta — dodaj pierwsze jedzenie!</p>
        </div>
      )}

      {cats.map((cat) => (
        <section class="food-group" key={cat}>
          <h3 class="group-title">
            {categoryEmoji(cat)} {categoryLabel(cat)}
            <span class="group-count">
              {byCat[cat].filter((f) => f.tries >= f.target).length}/{byCat[cat].length}
            </span>
          </h3>
          {byCat[cat].map((f) => (
            <div class="food-row" key={f.id} onClick={() => onOpenDetail(f.id)}>
              <div class="food-info">
                <div class="food-line">
                  <span class="food-name">{f.name}</span>
                  {f.rating_count > 0 && (
                    <span class="food-avg" title="średnia ocena">
                      {ratingMeta(Math.round(f.rating_avg)).emoji} {f.rating_avg.toFixed(1)}
                    </span>
                  )}
                </div>
                <div class="food-progress">
                  <div class="progress-track">
                    <div
                      class={f.tries >= f.target ? 'progress-fill done' : 'progress-fill'}
                      style={{ width: `${progressPct(f.tries, f.target)}%` }}
                    />
                  </div>
                  <span class="progress-label">
                    {f.tries >= f.target ? '🎉 cel' : `${f.tries}/${f.target} prób`}
                  </span>
                </div>
              </div>
              <div class="food-actions">
                <button
                  class="btn-minus"
                  aria-label="cofnij próbę"
                  onClick={(e) => {
                    e.stopPropagation();
                    onMinus(f);
                  }}
                >
                  −
                </button>
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
          ))}
        </section>
      ))}
    </div>
  );
}