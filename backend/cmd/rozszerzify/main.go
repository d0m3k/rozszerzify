package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"rozszerzify/internal/config"
	"rozszerzify/internal/db"
	"rozszerzify/internal/handlers"
	"rozszerzify/internal/middleware"
	"rozszerzify/internal/notify"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// starterFoods is installed on the very first boot when the food list is
// empty. Every entry starts at 0 tries — the parent fills in what the kid
// has already tried.
var starterFoods = []struct{ Name, Category string }{
	// alergeny (wprowadzać pojedynczo, po jednym na kilka dni,
	// w małych ilościach — najważniejsza kategoria)
	{"jajko (całe)", "alergeny"},
	{"orzeszki ziemne", "alergeny"},
	{"migdały", "alergeny"},
	{"orzechy laskowe", "alergeny"},
	{"sezam (tahini)", "alergeny"},
	{"mleko krowie (do picia)", "alergeny"},
	{"gluten (kasza manna)", "alergeny"},
	{"soja (tofu)", "alergeny"},
	{"krewetki", "alergeny"},
	// warzywa
	{"marchewka", "warzywa"},
	{"ziemniak", "warzywa"},
	{"brokuł", "warzywa"},
	{"kalafior", "warzywa"},
	{"dynia", "warzywa"},
	{"cukinia", "warzywa"},
	{"batat", "warzywa"},
	{"groszek zielony", "warzywa"},
	{"burak", "warzywa"},
	{"pietruszka", "warzywa"},
	{"awokado", "warzywa"},
	// owoce
	{"jabłko", "owoce"},
	{"gruszka", "owoce"},
	{"banan", "owoce"},
	{"morela", "owoce"},
	{"brzoskwinia", "owoce"},
	{"śliwka", "owoce"},
	{"malina", "owoce"},
	{"borówka", "owoce"},
	{"mango", "owoce"},
	// kasze i zboża
	{"kasza jaglana", "kasze i zboża"},
	{"kaszka ryżowa", "kasze i zboża"},
	{"kaszka kukurydziana", "kasze i zboża"},
	{"płatki owsiane", "kasze i zboża"},
	// mięso i ryby
	{"indyk", "mięso i ryby"},
	{"kurczak", "mięso i ryby"},
	{"cielęcina", "mięso i ryby"},
	{"łosoś", "mięso i ryby"},
	{"dorsz", "mięso i ryby"},
	{"żółtko jaja", "mięso i ryby"},
	// nabiał
	{"jogurt naturalny", "nabiał"},
	{"twarożek", "nabiał"},
	// inne
	{"oliwa z oliwek", "inne"},
	{"olej rzepakowy", "inne"},
	{"siemię lniane", "inne"},
}

func main() {
	seedFlag := flag.Bool("seed", false, "force seed even if data exists, then start")
	remindFlag := flag.Bool("remind", false, "cron mode: send start-date reminder if today is a checkpoint, then exit")
	flag.Parse()

	_ = godotenv.Load()

	cfg := config.Load()

	if *remindFlag {
		os.Exit(runReminder(cfg))
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}

	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer conn.Close()

	if *seedFlag || dbIsEmpty(conn) {
		if err := seedData(conn, cfg); err != nil {
			log.Printf("seed: %v (continuing anyway)", err)
		} else {
			log.Println("seed data ready")
		}
	}

	notifier := notify.New(cfg.PushoverUserKey, cfg.PushoverAppToken)
	if notifier.Enabled() {
		log.Println("notify: Pushover ENABLED (first-try + target-reached alerts)")
	} else {
		log.Println("notify: Pushover not configured — notifications disabled")
	}

	authH := &handlers.AuthHandler{DB: conn, Cfg: cfg}
	foodH := &handlers.FoodHandler{DB: conn, Cfg: cfg, Notify: notifier}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(corsMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", authH.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(cfg))

			r.Get("/foods", foodH.List)
			r.Post("/foods", foodH.Create)
			r.Get("/foods/{id}", foodH.Get)
			r.Put("/foods/{id}", foodH.Update)
			r.Delete("/foods/{id}", foodH.Delete)
			r.Post("/foods/{id}/try", foodH.Try)
			r.Post("/foods/{id}/untry", foodH.Untry)
			r.Get("/foods/{id}/log", foodH.Log)
			r.Delete("/foods/{id}/log/{logId}", foodH.DeleteLog)

			r.Get("/ranking", foodH.Ranking)
			r.Get("/stats", foodH.Stats)
		})
	})

	log.Printf("listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// runReminder is the cron job mode: it sends a Pushover notification on the
// countdown checkpoints (T-30/14/7/3/1/0) towards the diet start date.
// Idempotent via marker files, so a daily cron can call it freely.
func runReminder(cfg *config.Config) int {
	n := notify.New(cfg.PushoverUserKey, cfg.PushoverAppToken)
	if !n.Enabled() {
		fmt.Println("remind: Pushover not configured — nothing to do")
		return 0
	}

	nowY, nowM, nowD := time.Now().UTC().Date()
	today := time.Date(nowY, nowM, nowD, 0, 0, 0, 0, time.UTC)
	sY, sM, sD := cfg.StartTime().UTC().Date()
	start := time.Date(sY, sM, sD, 0, 0, 0, 0, time.UTC)

	daysUntil := int(start.Sub(today).Hours() / 24)
	if daysUntil < 0 {
		fmt.Printf("remind: diet already started %d days ago — no reminders\n", -daysUntil)
		return 0
	}

	checkpoints := map[int]string{
		30: "Do startu rozszerzania diety zostało 30 dni — planujesz pierwsze warzywa? 🥕",
		14: "Start diety za 14 dni — skompletuj łyżeczki, krzesełko i śliniaki 🍼",
		7:  "Za tydzień start rozszerzania diety! Przygotuj listę pierwszych smaków 🥦",
		3:  "72h do startu — zrób zapasy: marchew, ziemniak, dynia, brokuł 🛒",
		1:  "JUTRO start rozszerzania diety! Będzie się działo 🎉",
		0:  "DZIŚ start rozszerzania diety! Dzień 1 — powodzenia! 🍼✨",
	}

	msg, ok := checkpoints[daysUntil]
	if !ok {
		fmt.Printf("remind: no checkpoint today (T-%d)\n", daysUntil)
		return 0
	}

	// One push per checkpoint — marker files keep the daily cron idempotent.
	marker := filepath.Join(cfg.RemindDir, fmt.Sprintf(".remind-T%d", daysUntil))
	if _, err := os.Stat(marker); err == nil {
		fmt.Printf("remind: T-%d already sent\n", daysUntil)
		return 0
	}

	if err := os.MkdirAll(cfg.RemindDir, 0755); err != nil {
		fmt.Printf("remind: mkdir %v\n", err)
	}

	n.Send("⏰ Rozszerzanie diety", msg)
	_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)), 0644)
	fmt.Printf("remind: sent T-%d\n", daysUntil)
	return 0
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// dbIsEmpty reports whether the app has no user yet (fresh database).
func dbIsEmpty(db *sql.DB) bool {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rz_users`).Scan(&count); err != nil {
		return true
	}
	return count == 0
}

// seedData creates the single account (krzysio, password from
// SEED_PASSWORD — never commit the real one) and the starter list.
// Idempotent: skips work that is already there.
func seedData(conn *sql.DB, cfg *config.Config) error {
	// ── User ────────────────────────────────────────────────────────────
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM rz_users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	var uid int
	if count == 0 {
		if cfg.SeedPassword == "" {
			log.Println("SEED_PASSWORD not set — skipping user seed")
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("bcrypt: %w", err)
			}
			if err := conn.QueryRow(
				`INSERT INTO rz_users (username, password_hash) VALUES ('krzysio', $1)
				 ON CONFLICT (username) DO NOTHING RETURNING id`,
				string(hash),
			).Scan(&uid); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("create user: %w", err)
			}
			if uid > 0 {
				log.Println("  user krzysio created (password from SEED_PASSWORD)")
			}
		}
	}

	if uid == 0 {
		if err := conn.QueryRow(`SELECT id FROM rz_users WHERE username = 'krzysio'`).Scan(&uid); err != nil {
			return fmt.Errorf("lookup krzysio: %w", err)
		}
	}

	// ── Starter foods ───────────────────────────────────────────────────
	var foods int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM rz_foods WHERE user_id = $1`, uid).Scan(&foods); err != nil {
		return fmt.Errorf("count foods: %w", err)
	}
	if foods == 0 {
		for _, f := range starterFoods {
			if _, err := conn.Exec(
				`INSERT INTO rz_foods (user_id, name, category) VALUES ($1, $2, $3)
				 ON CONFLICT (user_id, name) DO NOTHING`,
				uid, f.Name, f.Category,
			); err != nil {
				log.Printf("  seed food %q: %v", f.Name, err)
			}
		}
		log.Printf("  %d starter foods added", len(starterFoods))
	}

	if cfg.StartDate != "" {
		log.Printf("  start date: %s", cfg.StartDate)
	}
	return nil
}