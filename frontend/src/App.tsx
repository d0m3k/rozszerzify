import { useCallback, useEffect, useState } from 'preact/hooks';
import { loadAuth, clearAuth, saveAuth, AuthState } from './stores/auth';
import { api, Food, RankingEntry, Stats } from './api';
import { LoginPage } from './pages/Login';
import { FoodsPage } from './pages/Foods';
import { FoodDetailPage } from './pages/FoodDetail';
import { AddFoodPage } from './pages/AddFood';
import { SearchPage } from './pages/Search';
import { RankingPage } from './pages/Ranking';
import { RatingSheet } from './components/RatingSheet';

type Page = 'foods' | 'ranking' | 'add' | 'search' | 'detail';

export function App() {
  const [auth, setAuth] = useState<AuthState | null>(loadAuth);
  const [page, setPage] = useState<Page>('foods');
  const [selId, setSelId] = useState(0);
  const [foods, setFoods] = useState<Food[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [ranking, setRanking] = useState<RankingEntry[]>([]);
  const [rankLoaded, setRankLoaded] = useState(false);
  const [sheetFor, setSheetFor] = useState<Food | null>(null);
  const [tick, setTick] = useState(0);

  const refresh = useCallback(() => setTick((t) => t + 1), []);

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
    setPage('foods');
  }

  function handleLogout() {
    clearAuth();
    setAuth(null);
  }

  async function handleTry(foodId: number, rating: number, note?: string) {
    try {
      await api.tryFood(foodId, rating, note);
    } catch {
      /* network hiccup — sheet stays? no, close and let user retry */
    } finally {
      setSheetFor(null);
      refresh();
    }
  }

  // Quick-add from search: create the food, then immediately open the rating
  // sheet for it so the parent can log the first try in one more tap.
  async function handleQuickAdd(name: string, category: string) {
    try {
      const created = await api.createFood({ name, category });
      setSheetFor(created);
      refresh();
    } catch {
      const existing = foods.find((f) => f.name.toLowerCase() === name.toLowerCase());
      if (existing) setSheetFor(existing);
    }
  }

  async function handleUntry(foodId: number) {
    try {
      await api.untryFood(foodId);
    } catch { /* ignore */ } finally {
      refresh();
    }
  }

  async function handleSave(foodId: number, patch: { notes?: string; target?: number }) {
    await api.updateFood(foodId, patch);
    refresh();
  }

  async function handleDelete(foodId: number) {
    await api.deleteFood(foodId);
    setPage('foods');
    refresh();
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
            onOpenDetail={(id) => { setSelId(id); setPage('detail'); }}
            onPlus={(f) => setSheetFor(f)}
            onMinus={(f) => handleUntry(f.id)}
          />
        )}
        {page === 'ranking' && (
          <RankingPage
            ranking={ranking}
            loading={!rankLoaded}
            onOpenDetail={(id) => { setSelId(id); setPage('detail'); }}
          />
        )}
        {page === 'search' && (
          <SearchPage
            foods={foods}
            onBack={() => setPage('foods')}
            onPick={(f) => setSheetFor(f)}
            onQuickAdd={handleQuickAdd}
            onFullForm={() => setPage('add')}
          />
        )}
        {page === 'add' && (
          <AddFoodPage
            onBack={() => setPage('foods')}
            onAdded={() => { setPage('foods'); refresh(); }}
          />
        )}
        {page === 'detail' && selFood && (
          <FoodDetailPage
            food={selFood}
            onBack={() => setPage('foods')}
            onPlus={() => setSheetFor(selFood)}
            onUntry={() => handleUntry(selFood.id)}
            onSave={(patch) => handleSave(selFood.id, patch)}
            onDelete={() => handleDelete(selFood.id)}
          />
        )}
      </div>

      <nav class="bottom-nav">
        <button class={page === 'foods' ? 'nav-btn active' : 'nav-btn'} onClick={() => setPage('foods')}>
          <span class="nav-icon">🍽️</span>
          <span>Jedzenie</span>
        </button>
        <button class="nav-add" onClick={() => setPage('search')} aria-label="szybkie dodawanie / wyszukiwanie">
          +
        </button>
        <button class={page === 'ranking' ? 'nav-btn active' : 'nav-btn'} onClick={() => setPage('ranking')}>
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
    </div>
  );
}