import { render } from 'preact';
import { useRegisterSW } from 'virtual:pwa-register/preact';
import { App } from './App';
import { InstallPrompt } from './components/InstallPrompt';
import './style.css';

let hadController = !!navigator.serviceWorker?.controller;

function Root() {
  const {
    needRefresh: [needRefresh, setNeedRefreshRaw],
    updateServiceWorker,
  } = useRegisterSW({
    onRegistered(swReg) {
      swReg?.update().catch(() => {});
      navigator.serviceWorker?.addEventListener('controllerchange', () => {
        if (hadController) window.location.reload();
        hadController = true;
      });
      if (swReg?.waiting) setNeedRefresh(true);
      swReg?.addEventListener('updatefound', () => {
        const newWorker = swReg.installing;
        if (newWorker) {
          newWorker.addEventListener('statechange', () => {
            if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
              setNeedRefresh(true);
            }
          });
        }
      });
    },
  });
  // vite-plugin-pwa types the setter as StateUpdater<T> (value union) instead
  // of Dispatch — it is a plain setter at runtime, so cast it.
  const setNeedRefresh = setNeedRefreshRaw as (v: boolean) => void;

  return (
    <>
      <App />
      <InstallPrompt />
      {needRefresh && (
        <div class="update-toast">
          <div class="update-toast-inner">
            <span>✨ Nowa wersja dostępna!</span>
            <button class="update-toast-btn" onClick={() => updateServiceWorker(true)}>
              Odśwież
            </button>
          </div>
        </div>
      )}
    </>
  );
}

render(<Root />, document.getElementById('app')!);