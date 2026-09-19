import { useState } from 'preact/hooks';
import { api } from '../api';
import { AuthState } from '../stores/auth';

interface Props {
  onLogin: (auth: AuthState) => void;
  onRegister: () => void;
  registerPrefill?: string;
}

const LAST_USER_KEY = 'rozszerzify_last_user';

export function LoginPage({ onLogin, onRegister, registerPrefill }: Props) {
  const [username, setUsername] = useState(registerPrefill || localStorage.getItem(LAST_USER_KEY) || '');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e: Event) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const auth = await api.login(username.trim(), password);
      localStorage.setItem(LAST_USER_KEY, username.trim());
      onLogin({ ...auth, userId: auth.user_id });
    } catch (err: any) {
      setError(err.message || 'Nie udało się zalogować');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div class="auth-wrap">
      <div class="auth-logo">🥣</div>
      <h1 class="auth-title">Rozszerzify</h1>
      <p class="auth-subtitle">
        Śledź rozszerzanie diety malucha.<br />
        Każde nowe jedzenie — jedna próba na raz. 👶
      </p>

      <form class="auth-form" onSubmit={submit}>
        <label class="field">
          <span class="field-label">Login</span>
          <input
            class="input"
            value={username}
            onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
            autoComplete="username"
            required
          />
        </label>
        <label class="field">
          <span class="field-label">Hasło</span>
          <input
            class="input"
            type="password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error && <div class="form-error">{error}</div>}
        <button class="btn-primary btn-block" type="submit" disabled={busy}>
          {busy ? 'Logowanie…' : 'Zaloguj się'}
        </button>

        <p class="auth-switch">
          Nowy maluch?{' '}
          <button type="button" class="link-btn" onClick={onRegister}>
            Zarejestruj konto
          </button>
        </p>
      </form>
    </div>
  );
}