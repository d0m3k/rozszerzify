import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';

interface Props {
  // back to login; passes the new username so the login form can prefill it
  onDone: (username?: string) => void;
  prefill?: string;
}

// Self-service registration (same flow as rybaspotting): login + password +
// the baby's birth date (+ optional diet start date), protected by
// Cloudflare Turnstile when the server has it configured.
export function RegisterPage({ onDone, prefill }: Props) {
  const [username, setUsername] = useState(prefill || '');
  const [password, setPassword] = useState('');
  const [birthDate, setBirthDate] = useState('');
  const [startDate, setStartDate] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  // Turnstile state — key empty = not configured (dev), widget hidden.
  const [tsKey, setTsKey] = useState('');
  const [tsToken, setTsToken] = useState('');
  const tsDivRef = useRef<HTMLDivElement | null>(null);
  const tsWidgetRef = useRef<any>(null);

  // Fetch public config once: decides whether the Turnstile widget shows.
  useEffect(() => {
    let cancelled = false;
    api.getConfig()
      .then(c => { if (!cancelled && c.turnstile_site_key) setTsKey(c.turnstile_site_key); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, []);

  // Render the Turnstile widget once the script + site key are available.
  useEffect(() => {
    if (!tsKey || !tsDivRef.current) return;
    let cancelled = false;

    const render = (): boolean => {
      const ts = (window as any).turnstile;
      if (!ts || !tsDivRef.current || cancelled || tsWidgetRef.current != null) return !!tsWidgetRef.current;
      tsWidgetRef.current = ts.render(tsDivRef.current, {
        sitekey: tsKey,
        theme: 'light',
        callback: (token: string) => setTsToken(token),
        'expired-callback': () => setTsToken(''),
        'error-callback': () => setTsToken(''),
      });
      return true;
    };

    if (!render()) {
      const iv = setInterval(() => { if (render()) clearInterval(iv); }, 200);
      return () => { cancelled = true; clearInterval(iv); };
    }
    return () => { cancelled = true; };
  }, [tsKey]);

  // Clean up the widget on unmount.
  useEffect(() => {
    return () => {
      const ts = (window as any).turnstile;
      if (ts && tsWidgetRef.current != null) {
        try { ts.remove(tsWidgetRef.current); } catch {}
      }
    };
  }, []);

  async function submit(e: Event) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      await api.register({
        username: username.trim(),
        password,
        birth_date: birthDate,          // may be empty → stats fall back / hide
        start_date: startDate || undefined,
        turnstile: tsKey ? tsToken : undefined,
      });
      setDone(true);
    } catch (err: any) {
      setError(err.message || 'Nie udało się zarejestrować');
      // Turnstile tokens are single-use — reset the widget so the user can retry.
      const ts = (window as any).turnstile;
      if (ts && tsWidgetRef.current != null) {
        try { ts.reset(tsWidgetRef.current); } catch {}
      }
      setTsToken('');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div class="auth-wrap">
      <div class="auth-logo">🍼</div>
      <h1 class="auth-title">Nowe konto</h1>
      <p class="auth-subtitle">
        Każde konto to jeden maluch — własna lista<br />
        produktów, prób i daty. 👶
      </p>

      {done ? (
        <div class="auth-form">
          <div class="form-success">
            ✓ Konto utworzone! Możesz się zalogować.
          </div>
          <button class="btn-primary btn-block" type="button" onClick={() => onDone(username.trim())}>
            Przejdź do logowania
          </button>
        </div>
      ) : (
        <form class="auth-form" onSubmit={submit}>
          <label class="field">
            <span class="field-label">Login</span>
            <input
              class="input"
              value={username}
              onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
              autoComplete="username"
              autoCapitalize="off"
              required
              minLength={2}
              maxLength={32}
            />
          </label>
          <label class="field">
            <span class="field-label">Hasło (min. 6 znaków)</span>
            <input
              class="input"
              type="password"
              value={password}
              onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
              autoComplete="new-password"
              required
              minLength={6}
            />
          </label>
          <label class="field">
            <span class="field-label">Data urodzenia malucha</span>
            <input
              class="input"
              type="date"
              value={birthDate}
              max={new Date().toISOString().slice(0, 10)}
              onInput={(e) => setBirthDate((e.target as HTMLInputElement).value)}
              required
            />
          </label>
          <label class="field">
            <span class="field-label">Start rozszerzania diety (opcjonalnie)</span>
            <input
              class="input"
              type="date"
              value={startDate}
              onInput={(e) => setStartDate((e.target as HTMLInputElement).value)}
            />
          </label>

          {tsKey && (
            <div
              ref={tsDivRef}
              style="display:flex;justify-content:center;min-height:65px;"
              aria-label="Weryfikacja botów"
            />
          )}

          {error && <div class="form-error">{error}</div>}
          <button
            class="btn-primary btn-block"
            type="submit"
            disabled={busy || (tsKey !== '' && tsToken === '')}
          >
            {busy ? 'Tworzę konto…' : (tsKey && !tsToken ? 'Potwierdź, że nie jesteś botem…' : 'Zarejestruj się')}
          </button>

          <p class="auth-switch">
            Masz już konto?{' '}
            <button type="button" class="link-btn" onClick={() => onDone(username.trim())}>
              Zaloguj się
            </button>
          </p>
        </form>
      )}
    </div>
  );
}
