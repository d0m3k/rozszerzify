// API client + shared types for Rozszerzify

export interface Food {
  id: number;
  name: string;
  category: string;
  tries: number;
  target: number;
  notes: string;
  created_at: string;
  updated_at: string;
  last_tried_at: string | null;
  rating_avg: number;
  rating_count: number;
  rating_sum: number;
  last_note: string;
}

export interface Stats {
  start_date: string;
  started: boolean;
  days_since_start: number;
  days_until_start: number;
  birth_date: string;
  baby_age_months: number;
  baby_age_days: number;
  foods_total: number;
  foods_at_target: number;
  tries_total: number;
  foods_tried_today: number;
}

export interface LogEntry {
  id: number;
  note: string;
  rating: number;
  tried_at: string;
}

export interface RankingEntry {
  food_id: number;
  name: string;
  category: string;
  tries: number;
  rating_count: number;
  rating_avg: number;
  rating_sum: number;
}

export interface AuthResponse {
  token: string;
  user_id: number;
  username: string;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const token = localStorage.getItem('raz_token');
  const headers: Record<string, string> = { ...opts.headers };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  if (opts.body && !(opts.body instanceof FormData)) headers['Content-Type'] = 'application/json';

  const res = await fetch(path, {
    method: opts.method || 'GET',
    headers,
    body: opts.body,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    if (res.status === 401) {
      localStorage.removeItem('raz_token');
    }
    throw new Error(err.error || 'Wystąpił błąd');
  }
  return res.json();
}

interface RequestOptions {
  method?: string;
  body?: any;
  headers?: Record<string, string>;
}

export const api = {
  login: (username: string, password: string) =>
    request<AuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  listFoods: () => request<Food[]>('/api/foods'),
  getFood: (id: number) => request<Food>(`/api/foods/${id}`),
  createFood: (data: { name: string; category: string; notes?: string; target?: number }) =>
    request<Food>('/api/foods', { method: 'POST', body: JSON.stringify(data) }),
  updateFood: (id: number, data: { name?: string; category?: string; notes?: string; target?: number }) =>
    request<Food>(`/api/foods/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteFood: (id: number) => request<{ ok: boolean }>(`/api/foods/${id}`, { method: 'DELETE' }),

  tryFood: (id: number, rating: number, note?: string) =>
    request<Food>(`/api/foods/${id}/try`, {
      method: 'POST',
      body: JSON.stringify({ rating, note: note || '' }),
    }),
  untryFood: (id: number) => request<Food>(`/api/foods/${id}/untry`, { method: 'POST' }),

  foodLog: (id: number) => request<LogEntry[]>(`/api/foods/${id}/log`),
  deleteLog: (foodId: number, logId: number) =>
    request<Food>(`/api/foods/${foodId}/log/${logId}`, { method: 'DELETE' }),
  ranking: () => request<RankingEntry[]>('/api/ranking'),
  stats: () => request<Stats>('/api/stats'),
};

// ── Data helpers ──────────────────────────────────────────────────────

export const RATINGS = [
  { v: 1, emoji: '😖', label: 'Nie zjadł' },
  { v: 2, emoji: '😐', label: 'Zjadł niechętnie' },
  { v: 3, emoji: '🙂', label: 'Zjadł' },
  { v: 4, emoji: '🤩', label: 'Zjadł i chciał więcej' },
];

export function ratingMeta(v: number) {
  return RATINGS.find((r) => r.v === v) ?? RATINGS[2];
}

export const CATEGORY_ORDER = ['warzywa', 'owoce', 'kasze i zboża', 'mięso i ryby', 'nabiał', 'inne'];

export const CATEGORIES: Record<string, { label: string; emoji: string }> = {
  'warzywa': { label: 'Warzywa', emoji: '🥕' },
  'owoce': { label: 'Owoce', emoji: '🍎' },
  'kasze i zboża': { label: 'Kasze i zboża', emoji: '🌾' },
  'mięso i ryby': { label: 'Mięso i ryby', emoji: '🍗' },
  'nabiał': { label: 'Nabiał', emoji: '🥛' },
  'inne': { label: 'Inne', emoji: '✨' },
};

export function categoryLabel(cat: string): string {
  return CATEGORIES[cat]?.label ?? cat;
}
export function categoryEmoji(cat: string): string {
  return CATEGORIES[cat]?.emoji ?? '✨';
}

// Per-food emoji, falling back to the category emoji.
const FOOD_EMOJI: Record<string, string> = {
  marchewka: '🥕', ziemniak: '🥔', brokuł: '🥦', kalafior: '🥦', dynia: '🎃',
  cukinia: '🥒', batat: '🍠', 'groszek zielony': '🟢', burak: '🌱', pietruszka: '🌿',
  awokado: '🥑',
  jabłko: '🍎', gruszka: '🍐', banan: '🍌', morela: '🍑', brzoskwinia: '🍑',
  śliwka: '🟣', malina: '🍓', borówka: '🫐', mango: '🥭',
  'kasza jaglana': '🌾', 'kaszka ryżowa': '🍚', 'kaszka kukurydziana': '🌽',
  'płatki owsiane': '🥣',
  indyk: '🦃', kurczak: '🍗', cielęcina: '🥩', łosoś: '🐟', dorsz: '🐟',
  'żółtko jaja': '🥚',
  'jogurt naturalny': '🥛', twarożek: '🧀',
  'oliwa z oliwek': '🫒', 'olej rzepakowy': '💧', 'siemię lniane': '🌰',
};

export function foodEmoji(name: string, category: string): string {
  return FOOD_EMOJI[name.toLowerCase()] ?? categoryEmoji(category);
}

export function sortCategories(list: string[]): string[] {
  const known = CATEGORY_ORDER.filter((c) => list.includes(c));
  const other = list.filter((c) => !CATEGORY_ORDER.includes(c)).sort();
  return [...known, ...other];
}

export function progressPct(tries: number, target: number): number {
  if (target <= 0) return 0;
  return Math.min(100, Math.round((tries / target) * 100));
}

export function fmtDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('pl-PL', { day: '2-digit', month: '2-digit', year: 'numeric' });
}
export function fmtDateTime(iso: string): string {
  const d = new Date(iso);
  return `${d.toLocaleDateString('pl-PL', { day: '2-digit', month: '2-digit' })} ${d.toLocaleTimeString('pl-PL', { hour: '2-digit', minute: '2-digit' })}`;
}