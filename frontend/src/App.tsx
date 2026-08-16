import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { loadAuth, clearAuth, saveAuth, AuthState } from './stores/auth';
import { api, Food, LogEntry, RankingEntry, Stats, normalize } from './api';
import { LoginPage } from './pages/Login';
import { FoodsPage } from './pages/Foods';
import { FoodDetailPage } from './pages/FoodDetail';
import { AddFoodPage } from './pages/AddFood';
import { SearchPage } from './pages/Search';
import { RankingPage } from './pages/Ranking';
import { RatingSheet } from './components/RatingSheet';

type Page = 'foods' | 'ranking' | 'add' | 'search' | 'detail';

interface Toast {
  msg: string;
  action?: { label: string; fn: () => void };
}

// ── Hash routing: #/foods, #/foods/{id}, #/ranking, #/add, #/search ─────
function parseHash(): { page: Page; id: number } {
  const h = window.location.hash.replace(/^#\/?/, '');
  const parts = h.split('/').filter(Boolean);
  if (parts[0] === 'foods' && parts[1]) {
    const id = parseInt(parts[1], 10);
    return { page: 'detail', id: Number.isFinite(id) ? id : 0 };
  }
  if (parts[0] === 'foods') return { page: 'foods', id: 0 };
  if (parts[0] === 'ranking' || parts[0] === 'add' || parts[0] === 'search') {
    return { page: parts[0], id: 0 };
  }
  return { page: 'foods', id: 0 };
}

export function App() {
  const init = parseHash();
  const [auth, setAuth] = useState<AuthState | null>(loadAuth);
  const [page, setPage] = useState<Page>(init.page);
  const [selId, setSelId] = useState(init.id);
  const [foods, setFoods] = useState<Food[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [ranking, setRanking] = useState<RankingEntry[]>([]);
  const [rankLoaded, setRankLoaded] = useState(false);
  const [sheetFor, setSheetFor] = useState<Food | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  const [tick, setTick] = useState(0);
  const toastTimer = useRef<number | null>(null);

  const refresh = useCallback(() => setTick((t) => t + 1), []);

  function showToast(msg: string, opts: { action?: Toast['action']; duration?: number } = {}) {
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    setToast({ msg, action: opts.action });
    toastTimer.current = window.setTimeout(() => setToast(null), opts.duration ?? 3500);
  }

  // Routing: state is driven by the hash (browser back/forward + deep links work).
  useEffect(() => {
    if (!window.location.hash) window.location.hash = '/foods';
    const onHash = () => {
      const { page: p, id } = parseHash();
      setPage(p);
      setSelId(id);
    };
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  function go(p: Page, id = 0) {
    const hash = p === 'detail' ? `#/foods/${id}` : `#/${p}`;
    if (hash === window.location.hash) {
      setPage(p);
      setSelId(id);
    } else {
      window.location.hash = hash;
    }
  }

  useEffect(() => {
    if (!auth) return;
    api.listFoods().then(setFoods).catch(() => {});
    api.stats().then(setStats).catch(() => {});
  }, [auth, tick]);

  useEffect(() => {
    if (!auth || page !== 'ranking') return;
    setRankLoaded(false);
    api.ranking().then((r) => { setRanking(r); setRankLoaded(true); }).catch(() => setRankLoaded(true));
  }, [auth, page, tick]);

  function handleLogin(state: AuthState) {
    saveAuth(state);
    setAuth(state);
    go('foods');
  }

  function handleLogout() {
    clearAuth();
    setAuth(null);
  }

  async function handleTry(foodId: number, rating: number, note?: string) {
    try {
      await api.tryFood(foodId, rating, note);
    } catch {
      /* ignore */
    } finally {
      setSheetFor(null);
      refresh();
    }
  }

  // Restore a removed try (undo) by re-adding it with the same rating + note.
  async function restoreTry(foodId: number, entry: LogEntry) {
    try {
      await api.tryFood(foodId, entry.rating, entry.note);
      showToast('✓ Próba przywrócona');
      refresh();
    } catch {
      showToast('Nie udało się przywrócić');
    }
  }

  // Removing one specific ✕ entry from the history log (undo offered).
  async function handleRemoveLog(foodId: number, entry: LogEntry) {
    try {
      await api.deleteLog(foodId, entry.id);
    } catch {
      return;
    }
    refresh();
    showToast('Usunięto próbę', {
      action: { label: 'Cofnij', fn: () => restoreTry(foodId, entry) },
      duration: 6000,
    });
  }

  async function handleSave(foodId: number, patch: { notes?: string; target?: number }) {
    await api.updateFood(foodId, patch);
    refresh();
    showToast('✓ Zapisano');
  }

  async function handleDelete(foodId: number) {
    await api.deleteFood(foodId);
    go('foods');
    refresh();
    showToast('Usunięto z listy');
  }

  async function handleQuickAdd(name: string, category: string) {
    try {
      const created = await api.createFood({ name, category });
      setSheetFor(created);
      refresh();
    } catch {
      const existing = foods.find((f) => normalize(f.name) === normalize(name));
      if (existing) setSheetFor(existing);
    }
  }

  if (!auth) {
    return (
      <div class="app-container">
        <div class="app-content">
          <LoginPage onLogin={handleLogin} />
        </div>
      </div>
    );
  }

  const selFood = foods.find((f) => f.id === selId);

  return (
    <div class="app-container">
      <header class="topbar">
        <div class="topbar-left">
          <span class="topbar-logo">🥣</span>
          <span class="topbar-title">Rozszerzify</span>
        </div>
        <button class="btn-ghost topbar-logout" onClick={handleLogout}>
          Wyloguj
        </button>
      </header>

      <div class="app-content">
        {page === 'foods' && (
          <FoodsPage
            foods={foods}
            stats={stats}
            onOpenDetail={(id) => go('detail', id)}
            onPlus={(f) => setSheetFor(f)}
          />
        )}
        {page === 'ranking' && (
          <RankingPage
            ranking={ranking}
            loading={!rankLoaded}
            onOpenDetail={(id) => go('detail', id)}
          />
        )}
        {page === 'search' && (
          <SearchPage
            foods={foods}
            onBack={() => go('foods')}
            onPick={(f) => setSheetFor(f)}
            onQuickAdd={handleQuickAdd}
            onFullForm={() => go('add')}
          />
        )}
        {page === 'add' && (
          <AddFoodPage
            onBack={() => go('foods')}
            onAdded={() => { go('foods'); refresh(); showToast('➕ Dodano'); }}
          />
        )}
        {page === 'detail' && selFood && (
          <FoodDetailPage
            food={selFood}
            onBack={() => go('foods')}
            onPlus={() => setSheetFor(selFood)}
            onRemoveLog={(entry) => handleRemoveLog(selFood.id, entry)}
            onSave={(patch) => handleSave(selFood.id, patch)}
            onDelete={() => handleDelete(selFood.id)}
          />
        )}
      </div>

      <nav class="bottom-nav">
        <button class={page === 'foods' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('foods')}>
          <span class="nav-icon">🍽️</span>
          <span>Jedzenie</span>
        </button>
        <button class="nav-add" onClick={() => go('search')} aria-label="szybkie dodawanie / wyszukiwanie">
          +
        </button>
        <button class={page === 'ranking' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('ranking')}>
          <span class="nav-icon">🏆</span>
          <span>Ranking</span>
        </button>
      </nav>

      {sheetFor && (
        <RatingSheet
          foodName={sheetFor.name}
          onSelect={(r, note) => handleTry(sheetFor.id, r, note)}
          onClose={() => setSheetFor(null)}
        />
      )}

      {toast && (
        <div class="toast">
          <span class="toast-msg">{toast.msg}</span>
          {toast.action && (
            <button
              class="toast-action"
              onClick={() => {
                toast.action!.fn();
                setToast(null);
              }}
            >
              {toast.action.label}
            </button>
          )}
        </div>
      )}
    </div>
  );
}