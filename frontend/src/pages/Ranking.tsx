import { RankingEntry, foodEmoji, ratingMeta } from '../api';

interface Props {
  ranking: RankingEntry[];
  loading: boolean;
  onOpenDetail: (foodId: number) => void;
}

const MEDALS = ['🥇', '🥈', '🥉'];

export function RankingPage({ ranking, loading, onOpenDetail }: Props) {
  const withRatings = ranking.filter((r) => r.rating_count > 0);
  const withoutRatings = ranking.filter((r) => r.rating_count === 0);

  return (
    <div class="page">
      <h2 class="page-title">🏆 Ranking smaków</h2>
      <p class="page-subtitle">
        Które jedzenie maluch lubi najbardziej? Średnia z ocen każdej próby.
      </p>

      {loading && <p class="muted">Ładowanie…</p>}

      {!loading && withRatings.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🤤</div>
          <p>
            Jeszcze nie ma ocen. Po każdym posiłku tapnij <b>+1</b> i wybierz, jak smakowało
            (od „nie smakuje” po „bardzo smakuje”), a ranking wypełni się sam.
          </p>
        </div>
      )}

      {withRatings.map((r, idx) => (
        <div class="rank-row" key={r.food_id} onClick={() => onOpenDetail(r.food_id)}>
          <span class="rank-medal">{MEDALS[idx] ?? `${idx + 1}.`}</span>
          <div class="rank-info">
            <div class="rank-name">
              {foodEmoji(r.name, r.category)} {r.name}
            </div>
            <div class="rank-track">
              <div
                class="rank-fill"
                style={{
                  width: `${Math.round((r.rating_avg / 4) * 100)}%`,
                  background: r.rating_avg >= 3.25 ? '#7BC950' : r.rating_avg >= 2.5 ? '#FFB300' : '#FF6B6B',
                }}
              />
            </div>
            <div class="rank-meta">
              <span class="rank-score">
                {ratingMeta(Math.round(r.rating_avg)).emoji} {r.rating_avg.toFixed(2)}
              </span>
              <span class="rank-count">
                {r.rating_count} {r.rating_count === 1 ? 'próba' : r.rating_count < 5 ? 'próby' : 'prób'}
                {r.tries - r.rating_count > 0 ? ` (+${r.tries - r.rating_count} bez oceny)` : ''}
              </span>
            </div>
          </div>
        </div>
      ))}

      {withoutRatings.length > 0 && (
        <section class="card">
          <h3 class="card-title">Jeszcze bez ocen</h3>
          <div class="unrated-list">
            {withoutRatings.slice(0, 20).map((r) => (
              <button key={r.food_id} class="unrated-chip" onClick={() => onOpenDetail(r.food_id)}>
                {categoryEmoji(r.category)} {r.name}
              </button>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}