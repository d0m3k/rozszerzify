package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func main() {
	seedFlag := flag.Bool("seed", false, "force seed even if data exists, then start")
	remindFlag := flag.Bool("remind", false, "cron mode: send start-date reminder if today is a checkpoint, then exit")
	newUser := flag.String("new-user", "", "create an additional account with this username, seed its starter foods, then exit")
	newPass := flag.String("new-pass", "", "password for -new-user (required with -new-user)")
	newBirth := flag.String("new-birth", "", "birth date YYYY-MM-DD for -new-user (optional)")
	newStart := flag.String("new-start", "", "diet start date YYYY-MM-DD for -new-user (optional)")
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

	// -new-user: create an additional account, then exit (does not start
	// the server — safe to run while the service is up).
	if *newUser != "" {
		os.Exit(createUser(conn, *newUser, *newPass, *newBirth, *newStart))
	}

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

	authH := &handlers.AuthHandler{DB: conn, Cfg: cfg, Notify: notifier}
	foodH := &handlers.FoodHandler{DB: conn, Cfg: cfg, Notify: notifier}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(corsMiddleware)

	r.Route("/api", func(r chi.Router) {
		// Public: /config exposes the Turnstile site key (when enabled);
		// registration is protected by Turnstile itself, login by credentials.
		r.Get("/config", authH.PublicConfig)
		r.Post("/auth/register", authH.Register)
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

	if err := db.SeedStarterFoods(conn, uid); err != nil {
		return err
	}

	if cfg.StartDate != "" {
		log.Printf("  start date: %s", cfg.StartDate)
	}
	return nil
}

// createUser is the -new-user CLI mode: creates an additional account
// (bcrypt password + optional birth/start dates) and seeds the same starter
// food list, so the new kid gets a separate set of trials. Exits 0 on
// success, 1 on failure. Safe to run while the service is up.
// Usage on the server:
//   cd /opt/rozszerzify && ./rozszerzify -new-user <name> -new-pass <pass> \
//     [-new-birth YYYY-MM-DD] [-new-start YYYY-MM-DD]
func createUser(conn *sql.DB, username, password, birthDate, startDate string) int {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		fmt.Println("new-user: username is required")
		return 1
	}
	if password == "" {
		fmt.Printf("new-user: password is required for %q\n", username)
		return 1
	}
	for _, d := range []struct{ label, val string }{{"birth", birthDate}, {"start", startDate}} {
		if d.val != "" {
			if _, err := time.Parse("2006-01-02", d.val); err != nil {
				fmt.Printf("new-user: bad %s date %q (want YYYY-MM-DD)\n", d.label, d.val)
				return 1
			}
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("new-user: bcrypt: %v\n", err)
		return 1
	}

	var uid int
	err = conn.QueryRow(
		`INSERT INTO rz_users (username, password_hash, birth_date, start_date)
			 VALUES ($1, $2, NULLIF($3, '')::date, NULLIF($4, '')::date)
			 RETURNING id`,
		username, string(hash), birthDate, startDate,
	).Scan(&uid)
	if err != nil {
		fmt.Printf("new-user: create %q: %v (already exists?)\n", username, err)
		return 1
	}
	fmt.Printf("new-user: account %q created (id=%d)\n", username, uid)

	if err := db.SeedStarterFoods(conn, uid); err != nil {
		fmt.Printf("new-user: seed foods: %v\n", err)
		return 1
	}
	fmt.Printf("new-user: done — %d starter foods, birth=%q start=%q\n", len(db.StarterFoods), birthDate, startDate)
	return 0
}