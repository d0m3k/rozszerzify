import { useEffect, useState } from 'preact/hooks';
import {
  Category,
  RecentLogEntry,
  api,
  foodEmoji,
  ratingMeta,
} from '../api';

interface Props {
  /** Bumped by App after any mutation (try, undo, delete) so the feed reloads. */
  tick: number;
  categories: Category[];
  onOpenDetail: (foodId: number) => void;
}

const PAGE_SIZE = 50;

function dayLabel(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const day = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  if (day === today) return 'Dzisiaj';
  if (day === today - 86400000) return 'Wczoraj';
  const label = d.toLocaleDateString('pl-PL', { weekday: 'long', day: 'numeric', month: 'long' });
  return label.charAt(0).toUpperCase() + label.slice(1);
}

export function RecentPage({ tick, categories, onOpenDetail }: Props) {
  const [entries, setEntries] = useState<RecentLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api
      .recentLog(PAGE_SIZE)
      .then((rows) => {
        if (!alive) return;
        setEntries(rows);
        setHasMore(rows.length === PAGE_SIZE);
        setError('');
      })
      .catch((err) => {
        if (alive) setError(err instanceof Error ? err.message : 'Nie udało się pobrać historii');
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [tick]);

  async function loadMore() {
    setLoadingMore(true);
    try {
      const rows = await api.recentLog(PAGE_SIZE, entries.length);
      setHasMore(rows.length === PAGE_SIZE);
      // A try added while paging could shift the offset; drop duplicates.
      setEntries((prev) => {
        const seen = new Set(prev.map((e) => e.id));
        return [...prev, ...rows.filter((e) => !seen.has(e.id))];
      });
    } catch {
      /* keep what is already shown */
    } finally {
      setLoadingMore(false);
    }
  }

  const groups: [string, RecentLogEntry[]][] = [];
  for (const e of entries) {
    const d = new Date(e.tried_at);
    const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
    const last = groups[groups.length - 1];
    if (last && last[0] === key) last[1].push(e);
    else groups.push([key, [e]]);
  }

  return (
    <div class="page">
      <h2 class="page-title">🕒 Ostatnie próby</h2>
      <p class="page-subtitle">
        Wszystko, co maluch jadł — każde jedzenie w jednym miejscu. Tapnij wpis, żeby otworzyć produkt.
      </p>

      {loading && <p class="muted">Ładowanie…</p>}

      {!loading && error && <p class="form-error">{error}</p>}

      {!loading && !error && entries.length === 0 && (
        <div class="empty-state">
          <div class="empty-emoji">🍽️</div>
          <p>
            Jeszcze nie ma prób. Tapnij <b>+1</b> po każdym podaniu jedzenia, a tutaj pojawi się
            cała historia — z datą, oceną i notatką.
          </p>
        </div>
      )}

      {groups.map(([key, rows]) => (
        <section class="recent-day" key={key}>
          <h3 class="recent-day-title">
            {dayLabel(rows[0].tried_at)}
            <span class="recent-day-count">
              {rows.length} {rows.length === 1 ? 'próba' : rows.length < 5 ? 'próby' : 'prób'}
            </span>
          </h3>
          <div class="recent-list">
            {rows.map((e) => {
              const meta = ratingMeta(e.rating);
              return (
                <button
                  class="recent-row"
                  key={e.id}
                  onClick={() => onOpenDetail(e.food_id)}
                  title={`Otwórz ${e.food_name}`}
                >
                  <span class="recent-food-icon" aria-hidden="true">
                    {e.food_emoji || foodEmoji(e.food_name, e.category, categories)}
                  </span>
                  <span class="recent-body">
                    <span class="recent-name">{e.food_name}</span>
                    <span class="recent-meta">
                      {meta.emoji} {e.note || meta.label.toLowerCase()}
                    </span>
                  </span>
                  <span class="recent-time">
                    {new Date(e.tried_at).toLocaleTimeString('pl-PL', { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </button>
              );
            })}
          </div>
        </section>
      ))}

      {!loading && hasMore && entries.length > 0 && (
        <button class="btn-ghost recent-more" onClick={loadMore} disabled={loadingMore}>
          {loadingMore ? 'Ładowanie…' : 'Pokaż starsze'}
        </button>
      )}
    </div>
  );
}
