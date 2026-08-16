# 🥣 Rozszerzify — dieta niemowlaka

PWA do śledzenia rozszerzania diety malucha. Prosty workflow: po każdym
podaniu nowego jedzenia tapnij **+1** i zaznacz, jak smakowało. Po 15
próbach (cel domyślny) wiesz, czy dziecko coś lubi. Ranking smaków pokazuje,
co mu najbardziej podchodzi.

## Stack

| Warstwa | Technologia |
|---|---|
| Backend | Go + chi + lib/pq + JWT (bcrypt) |
| Frontend | Preact + Vite + vite-plugin-pwa |
| Baza | wspólny PostgreSQL mikr.us (`psql01.mikr.us`, tabele `rz_*` obok rybaspotting) |
| Deploy | nginx + systemd + Cloudflare Tunnel (rozszerzify.dom3k.pl) |

## Funkcje

- **+1 po każdym posiłku** — duży przycisk, zero klikania w menu
- **Ocena próby 1–4**: 😖 nie smakuje · 😐 zjada, ale niechętnie · 🙂 smakuje · 🤩 bardzo smakuje
- **Cel prób** (domyślnie 15) — pasek postępu na każdym produkcie
- **🏆 Ranking smaków** — średnia ocen, medale dla top 3
- **Historia prób** — kiedy, z jaką notatką i oceną
- **PWA** — instalacja na ekran główny, offline shell
- **Jedno konto**: `krzysio` (hasło z `SEED_PASSWORD` w `.env` instancji)

## Szybki start (lokalnie)

```bash
cp .env.example .env        # uzupełnij DATABASE_URL (wspólny psql01.mikr.us)
cd backend && go run ./cmd/rozszerzify/     # API na :8081
cd frontend && npm install && npm run dev   # UI na :5173 (proxy /api → :8081)
```

Przy pierwszym uruchomieniu na pustej bazie serwer sam tworzy konto
`krzysio` (hasło: `SEED_PASSWORD` z `.env` — NIE commitować) oraz listę
startową ~35 produktów (wszystkie z `tries: 0`).

## Deploy na VPS

```bash
./scripts/build.sh    # backend linux/amd64 + frontend PWA
./scripts/deploy.sh   # scp na 'amy' (alias z ~/.ssh/config), systemd + nginx
```

Przygotowanie na VPS raz:
1. Utwórz `/opt/rozszerzify/.env` (wzór w `scripts/deploy.sh`) — sekrety są
   pisane bezpośrednio na serwerze, nigdy nie trafiają do repo.
2. Cloudflare Dashboard → **Zero Trust → Tunnels → public hostnames**:
   dodaj `rozszerzify.dom3k.pl` → typ HTTP → URL `localhost:80`
   (ten sam tunel co `ryby.dom3k.pl`, ruch idzie przez nginx, który
   rozróżnia po `server_name`).

Backend słucha na `127.0.0.1:8081` (8080 zajmuje rybaspotting).

## API

| Metoda | Ścieżka | Opis |
|---|---|---|
| POST | `/api/auth/login` | login `krzysio` → JWT (bezterminowy) |
| GET | `/api/foods` | lista produktów (z agregatami ocen) |
| POST | `/api/foods` | dodaj produkt |
| GET/PUT/DELETE | `/api/foods/{id}` | szczegóły / edycja / usunięcie |
| POST | `/api/foods/{id}/try` | `{rating: 1–4, note?}` — zarejestruj próbę |
| POST | `/api/foods/{id}/untry` | cofnij ostatnią próbę (pomyłka) |
| GET | `/api/foods/{id}/log` | historia prób |
| GET | `/api/ranking` | produkty wg średniej oceny |
| GET | `/api/stats` | liczniki: dzień od startu, produkty, cele |