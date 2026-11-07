import { Category, Meal, mealIcon, mealItemIcon, progressPct, ratingMeta, subMealIcon } from '../api';

interface Props {
  meals: Meal[];
  categories: Category[];
  onOpenDetail: (id: number) => void;
  onPlus: (meal: Meal) => void;
  onNew: () => void;
}

export function MealsPage({ meals, categories, onOpenDetail, onPlus, onNew }: Props) {
  return (
    <div class="page">
      <div class="meals-head">
        <div>
          <h2 class="page-title">🍲 Dania</h2>
          <p class="page-subtitle">
            Danie to gotowy zestaw produktów. Zjedzenie dania dodaje próbę daniu <b>i każdemu jego produktowi</b>.
          </p>
        </div>
        <button class="btn-primary meals-new" onClick={onNew}>
          ＋ Nowe danie
        </button>
      </div>

      {meals.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍲</div>
          <p>
            Nie masz jeszcze żadnego dania. Złóż je z produktów, które maluch je razem — np. „kasza z
            marchewką” — i loguj cały posiłek jednym tapnięciem.
          </p>
          <button class="btn-primary" onClick={onNew}>
            ＋ Złóż pierwsze danie
          </button>
        </div>
      )}

      {meals.map((m) => {
        const profile = m.emoji ? 'meal-emoji' : 'meal-emoji meal-emoji-default';
        return (
          <div class="food-row meal-row" key={m.id} onClick={() => onOpenDetail(m.id)}>
            <div class="food-info">
              <div class="food-line">
                <span class="food-name">
                  <span class={profile} aria-hidden="true">{mealIcon(m)}</span> {m.name}
                </span>
                {m.last_rating > 0 && m.tries > 0 && (
                  <span class="food-last-emoji" title="ostatnia próba">{ratingMeta(m.last_rating).emoji}</span>
                )}
              </div>
              <div class="meal-composition">
                <span class="meal-composition-label">Skład:</span>
                {m.items.map((it) => (
                  <span class="meal-item-chip" key={it.food_id}>
                    {mealItemIcon(it, categories)} {it.name}
                  </span>
                ))}
                {m.sub_meals.map((sm) => (
                  <span class="meal-item-chip meal-item-sub" key={sm.meal_id}>
                    {subMealIcon(sm)} {sm.name}
                    <span class="meal-item-sub-tag">danie</span>
                  </span>
                ))}
              </div>
              <div class="food-progress">
                <div class="progress-track">
                  <div
                    class={m.tries >= m.target ? 'progress-fill done' : 'progress-fill'}
                    style={{ width: `${progressPct(m.tries, m.target)}%` }}
                  />
                </div>
                <span class="progress-label">
                  {m.tries >= m.target ? '🎉 cel' : `${m.tries}/${m.target} prób`}
                </span>
              </div>
              <div class="meal-count-line">
                {m.items.length} {m.items.length === 1 ? 'produkt' : m.items.length < 5 ? 'produkty' : 'produktów'}
                {m.sub_meals.length > 0 && <>
                  {' '}+ {m.sub_meals.length} {m.sub_meals.length === 1 ? 'danie' : 'dania'}
                </>}{' '}
                w daniu · 1 próba = {m.expanded_foods} {m.expanded_foods === 1 ? 'produkt' : 'produktów'}
                {m.last_note ? ` · 📝 ${m.last_note}` : ''}
              </div>
            </div>
            <div class="food-actions">
              <button
                class="btn-plus"
                aria-label="dodaj próbę dania"
                onClick={(e) => {
                  e.stopPropagation();
                  onPlus(m);
                }}
              >
                <span class="btn-plus-plus">+1</span>
                <span class="btn-plus-sub">zjedzone</span>
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
