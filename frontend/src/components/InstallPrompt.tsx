import { useEffect, useState } from 'preact/hooks';

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

export function InstallPrompt() {
  const [deferred, setDeferred] = useState<BeforeInstallPromptEvent | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    const onPrompt = (e: Event) => {
      e.preventDefault();
      setDeferred(e as BeforeInstallPromptEvent);
    };
    window.addEventListener('beforeinstallprompt', onPrompt);
    return () => window.removeEventListener('beforeinstallprompt', onPrompt);
  }, []);

  if (!deferred || dismissed) return null;

  return (
    <div class="install-prompt">
      <div class="install-prompt-body">
        <span>🍼 Dodaj Rozszerzify do ekranu głównego — logowanie w jednym dotknięciu.</span>
        <div class="install-prompt-actions">
          <button
            class="btn-primary"
            onClick={() => {
              deferred.prompt();
              deferred.userChoice.then(() => setDeferred(null));
            }}
          >
            Zainstaluj
          </button>
          <button class="btn-ghost" onClick={() => setDismissed(true)}>
            Nie teraz
          </button>
        </div>
      </div>
    </div>
  );
}