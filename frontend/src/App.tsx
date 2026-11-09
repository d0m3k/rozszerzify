import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { loadAuth, clearAuth, saveAuth, AuthState } from './stores/auth';
import { api, Category, Food, LogEntry, Meal, RankingEntry, Stats, mealItemIcon, normalize, subMealIcon } from './api';
import { LoginPage } from './pages/Login';
import { RegisterPage } from './pages/Register';
import { FoodsPage } from './pages/Foods';
import { FoodDetailPage } from './pages/FoodDetail';
import { AddFoodPage } from './pages/AddFood';
import { SearchPage } from './pages/Search';
import { RankingPage } from './pages/Ranking';
import { RecentPage } from './pages/Recent';
import { MealsPage } from './pages/Meals';
import { MealDetailPage } from './pages/MealDetail';
import { AddMealPage } from './pages/AddMeal';
import { RatingSheet } from './components/RatingSheet';

type Page = 'foods' | 'meals' | 'meal' | 'add-meal' | 'ranking' | 'recent' | 'add' | 'search' | 'detail';

interface Toast {
  msg: string;
  action?: { label: string; fn: () => void };
}

// ── Hash routing: #/foods, #/foods/{id}, #/meals, #/meals/{id},
//    #/meals/new, #/ranking, #/recent, #/add, #/search ───────────────────
function parseHash(): { page: Page; id: number } {
  const h = window.location.hash.replace(/^#\/?/, '');
  const parts = h.split('/').filter(Boolean);
  if (parts[0] === 'foods' && parts[1]) {
    const id = parseInt(parts[1], 10);
    return { page: 'detail', id: Number.isFinite(id) ? id : 0 };
  }
  if (parts[0] === 'foods') return { page: 'foods', id: 0 };
  if (parts[0] === 'meals' && parts[1] === 'new') return { page: 'add-meal', id: 0 };
  if (parts[0] === 'meals' && parts[1]) {
    const id = parseInt(parts[1], 10);
    return { page: 'meal', id: Number.isFinite(id) ? id : 0 };
  }
  if (parts[0] === 'meals') return { page: 'meals', id: 0 };
  if (parts[0] === 'ranking' || parts[0] === 'recent' || parts[0] === 'add' || parts[0] === 'search') {
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
  const [meals, setMeals] = useState<Meal[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [ranking, setRanking] = useState<RankingEntry[]>([]);
  const [rankLoaded, setRankLoaded] = useState(false);
  const [sheetFor, setSheetFor] = useState<Food | null>(null);
  const [mealSheetFor, setMealSheetFor] = useState<Meal | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  const [tick, setTick] = useState(0);
  const [showRegister, setShowRegister] = useState(false);
  const [registerPrefill, setRegisterPrefill] = useState('');
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
    const hash = p === 'detail' ? `#/foods/${id}` : p === 'meal' ? `#/meals/${id}` : p === 'add-meal' ? '#/meals/new' : `#/${p}`;
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
    api.listMeals().then(setMeals).catch(() => {});
    api.listCategories().then(setCategories).catch(() => {});
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

  async function handleTry(foodId: number, rating: number, note?: string, triedAt?: string) {
    try {
      await api.tryFood(foodId, rating, note, triedAt);
    } catch {
      /* ignore */
    } finally {
      setSheetFor(null);
      refresh();
    }
  }

  async function handleTryMeal(meal: Meal, rating: number, note?: string, triedAt?: string) {
    try {
      await api.tryMeal(meal.id, rating, note, triedAt);
      const parts = [
        `${meal.expanded_foods} ${meal.expanded_foods === 1 ? 'produkt' : 'produkty'}`,
        ...(meal.expanded_meals > 0
          ? [`${meal.expanded_meals} ${meal.expanded_meals === 1 ? 'danie-składnik' : 'dania-składniki'}`]
          : []),
      ];
      showToast(`✓ ${meal.name} — +1 danie i ${parts.join(' + ')}`);
    } catch {
      /* ignore */
    } finally {
      setMealSheetFor(null);
      refresh();
    }
  }

  // Restore a removed try (undo) by re-adding it with the same rating, note
  // and original timestamp.
  async function restoreTry(foodId: number, entry: LogEntry) {
    try {
      await api.tryFood(foodId, entry.rating, entry.note, entry.tried_at);
      showToast('✓ Próba przywrócona');
      refresh();
    } catch {
      showToast('Nie udało się przywrócić');
    }
  }

  // Removing one specific ✕ entry from the meal history (undo offered). A meal
  // event is restored with the same rating, note and timestamp, which also
  // re-adds the ingredient tries.
  async function restoreMealTry(mealId: number, entry: LogEntry) {
    try {
      await api.tryMeal(mealId, entry.rating, entry.note, entry.tried_at);
      showToast('✓ Próba dania przywrócona');
      refresh();
    } catch {
      showToast('Nie udało się przywrócić');
    }
  }

  async function handleRemoveMealLog(mealId: number, entry: LogEntry) {
    try {
      await api.deleteMealLog(mealId, entry.id);
    } catch {
      return;
    }
    refresh();
    showToast('Usunięto próbę dania (i próby produktów)', {
      action: { label: 'Cofnij', fn: () => restoreMealTry(mealId, entry) },
      duration: 6000,
    });
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

  async function handleSave(foodId: number, patch: { emoji?: string; categories?: string[]; notes?: string; target?: number }) {
    await api.updateFood(foodId, patch);
    refresh();
    showToast('✓ Zapisano');
  }

  async function handleSaveMeal(
    mealId: number,
    patch: { name?: string; emoji?: string; notes?: string; target?: number; items?: number[]; sub_meals?: number[] },
  ) {
    await api.updateMeal(mealId, patch);
    refresh();
    showToast('✓ Zapisano danie');
  }

  // Creating a category inline (food forms) returns it so the form can select it.
  async function handleCreateCategory(name: string): Promise<Category> {
    const created = await api.createCategory(name);
    setCategories((prev) => [...prev, created].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'pl')));
    return created;
  }

  async function handleRenameCategory(id: number, name: string, emoji: string) {
    await api.updateCategory(id, { name, emoji });
    refresh();
    showToast('✓ Kategoria zmieniona');
  }

  async function handleDeleteCategory(id: number) {
    await api.deleteCategory(id);
    refresh();
    showToast('Kategoria usunięta');
  }

  async function handleDelete(foodId: number) {
    await api.deleteFood(foodId);
    go('foods');
    refresh();
    showToast('Usunięto z listy');
  }

  async function handleDeleteMeal(mealId: number) {
    await api.deleteMeal(mealId);
    go('meals');
    refresh();
    showToast('Usunięto danie');
  }

  async function handleQuickAdd(name: string, cats: string[]) {
    try {
      const created = await api.createFood({ name, categories: cats });
      setSheetFor(created);
      refresh();
    } catch {
      const existing = foods.find((f) => normalize(f.name) === normalize(name));
      if (existing) setSheetFor(existing);
    }
  }

  // Creating a product from inside a meal's composition picker: the picker
  // needs the created food back so it can select it immediately.
  async function handleCreateFood(input: { name: string; category: string }): Promise<Food> {
    const created = await api.createFood({ name: input.name, categories: [input.category] });
    setFoods((prev) => [...prev, created].sort((a, b) => a.name.localeCompare(b.name, 'pl')));
    refresh();
    showToast(`➕ Dodano produkt „${created.name}” do składu`);
    return created;
  }

  if (!auth) {
    return (
      <div class="app-container">
        <div class="app-content">
          {showRegister ? (
            <RegisterPage
              prefill={registerPrefill}
              onDone={(u) => {
                if (u) setRegisterPrefill(u);
                setShowRegister(false);
              }}
            />
          ) : (
            <LoginPage
              onLogin={handleLogin}
              onRegister={() => setShowRegister(true)}
              registerPrefill={registerPrefill}
            />
          )}
        </div>
      </div>
    );
  }

  const selFood = foods.find((f) => f.id === selId);
  const selMeal = meals.find((m) => m.id === selId);

  return (
    <div class="app-container">
      <header class="topbar">
        <div class="topbar-left">
          <span class="topbar-logo">🥣</span>
          <span class="topbar-title">Rozszerzify</span>
          <span class="topbar-user">{auth.username}</span>
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
            categories={categories}
            onOpenDetail={(id) => go('detail', id)}
            onPlus={(f) => setSheetFor(f)}
            onNew={() => go('add')}
            onRenameCategory={handleRenameCategory}
            onDeleteCategory={handleDeleteCategory}
          />
        )}
        {page === 'meals' && (
          <MealsPage
            meals={meals}
            categories={categories}
            onOpenDetail={(id) => go('meal', id)}
            onPlus={(m) => setMealSheetFor(m)}
            onNew={() => go('add-meal')}
          />
        )}
        {page === 'add-meal' && (
          <AddMealPage
            foods={foods}
            meals={meals}
            categories={categories}
            onBack={() => go('meals')}
            onAdded={() => { go('meals'); refresh(); showToast('➕ Dodano danie'); }}
            onCreateFood={handleCreateFood}
          />
        )}
        {page === 'meal' && selMeal && (
          <MealDetailPage
            meal={selMeal}
            foods={foods}
            meals={meals}
            categories={categories}
            onBack={() => go('meals')}
            onPlus={() => setMealSheetFor(selMeal)}
            onRemoveLog={(entry) => handleRemoveMealLog(selMeal.id, entry)}
            onSave={(patch) => handleSaveMeal(selMeal.id, patch)}
            onDelete={() => handleDeleteMeal(selMeal.id)}
            onOpenFood={(id) => go('detail', id)}
            onOpenMeal={(id) => go('meal', id)}
            onCreateFood={handleCreateFood}
          />
        )}
        {page === 'ranking' && (
          <RankingPage
            ranking={ranking}
            loading={!rankLoaded}
            onOpenDetail={(id) => go('detail', id)}
          />
        )}
        {page === 'recent' && (
          <RecentPage
            tick={tick}
            categories={categories}
            onOpenDetail={(id) => go('detail', id)}
          />
        )}
        {page === 'search' && (
          <SearchPage
            foods={foods}
            categories={categories}
            onBack={() => go('foods')}
            onPick={(f) => setSheetFor(f)}
            onQuickAdd={handleQuickAdd}
            onFullForm={() => go('add')}
          />
        )}
        {page === 'add' && (
          <AddFoodPage
            categories={categories}
            onBack={() => go('foods')}
            onAdded={() => { go('foods'); refresh(); showToast('➕ Dodano'); }}
            onCreateCategory={handleCreateCategory}
          />
        )}
        {page === 'detail' && selFood && (
          <FoodDetailPage
            food={selFood}
            categories={categories}
            onBack={() => go('foods')}
            onPlus={() => setSheetFor(selFood)}
            onRemoveLog={(entry) => handleRemoveLog(selFood.id, entry)}
            onSave={(patch) => handleSave(selFood.id, patch)}
            onCreateCategory={handleCreateCategory}
            onDelete={() => handleDelete(selFood.id)}
          />
        )}
      </div>

      <nav class="bottom-nav">
        <button class={page === 'foods' || page === 'detail' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('foods')}>
          <span class="nav-icon">🍽️</span>
          <span>Jedzenie</span>
        </button>
        <button class={page === 'meals' || page === 'meal' || page === 'add-meal' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('meals')}>
          <span class="nav-icon">🍲</span>
          <span>Dania</span>
        </button>
        <button class="nav-add" onClick={() => go('search')} aria-label="szybkie dodawanie / wyszukiwanie">
          +
        </button>
        <button class={page === 'recent' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('recent')}>
          <span class="nav-icon">🕒</span>
          <span>Ostatnie</span>
        </button>
        <button class={page === 'ranking' ? 'nav-btn active' : 'nav-btn'} onClick={() => go('ranking')}>
          <span class="nav-icon">🏆</span>
          <span>Ranking</span>
        </button>
      </nav>

      {sheetFor && (
        <RatingSheet
          foodName={sheetFor.name}
          subtitle={
            sheetFor.meal_names.length > 0
              ? `🍲 produkt wchodzi w skład dań: ${sheetFor.meal_names.join(' · ')}`
              : undefined
          }
          onSelect={(r, note, triedAt) => handleTry(sheetFor.id, r, note, triedAt)}
          onClose={() => setSheetFor(null)}
        />
      )}

      {mealSheetFor && (
        <RatingSheet
          foodName={mealSheetFor.name}
          subtitle={`🍲 danie z: ${[
            ...mealSheetFor.items.map((it) => `${mealItemIcon(it, categories)} ${it.name}`),
            ...mealSheetFor.sub_meals.map((sm) => `${subMealIcon(sm)} ${sm.name}`),
          ].join(' · ')}`}
          onSelect={(r, note, triedAt) => handleTryMeal(mealSheetFor, r, note, triedAt)}
          onClose={() => setMealSheetFor(null)}
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