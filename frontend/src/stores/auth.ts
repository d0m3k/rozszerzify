// Simple auth store backed by localStorage

export interface AuthState {
  token: string;
  userId: number;
  username: string;
}

const AUTH_KEY = 'rozszerzify_auth';

export function loadAuth(): AuthState | null {
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return null;
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

export function saveAuth(state: AuthState) {
  localStorage.setItem(AUTH_KEY, JSON.stringify(state));
  localStorage.setItem('raz_token', state.token);
}

export function clearAuth() {
  localStorage.removeItem(AUTH_KEY);
  localStorage.removeItem('raz_token');
}