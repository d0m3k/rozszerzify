package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"rozszerzify/internal/config"
	"rozszerzify/internal/db"
	"rozszerzify/internal/middleware"
	"rozszerzify/internal/notify"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB     *sql.DB
	Cfg    *config.Config
	Notify *notify.Notifier
}

type registerRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	BirthDate string `json:"birth_date"` // baby's birthday, YYYY-MM-DD
	StartDate string `json:"start_date"` // diet expansion start, optional
	Turnstile string `json:"turnstile"`  // Cloudflare Turnstile token
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// PublicConfig exposes non-secret runtime config to the frontend. The
// Turnstile site key is only advertised when verification is actually
// enabled (secret set), so the frontend knows whether to render the widget.
func (h *AuthHandler) PublicConfig(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{}
	if h.Cfg.TurnstileSecret != "" && h.Cfg.TurnstileSiteKey != "" {
		resp["turnstile_site_key"] = h.Cfg.TurnstileSiteKey
	}
	writeJSON(w, http.StatusOK, resp)
}

// Register is the self-service account creation (same flow rybaspotting
// uses): Cloudflare Turnstile when configured, bcrypt password, per-baby
// birth/start dates, starter food list, Pushover notification on success.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	if len(username) < 2 || len(username) > 32 {
		writeErr(w, http.StatusBadRequest, "login musi mieć 2–32 znaki")
		return
	}
	if len(req.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "hasło musi mieć min. 6 znaków")
		return
	}

	// Bot protection. When Turnstile is configured (secret set) a verified
	// token is required; without it (local/dev) registration stays open.
	if h.Cfg.TurnstileSecret != "" {
		ok, err := VerifyTurnstile(h.Cfg.TurnstileSecret, req.Turnstile, clientIP(r))
		if err != nil {
			log.Printf("[AUTH] turnstile verify error for user=%s: %v", username, err)
			writeErr(w, http.StatusInternalServerError, "captcha verification failed")
			return
		}
		if !ok {
			log.Printf("[AUTH] turnstile rejected for user=%s from %s", username, clientIP(r))
			writeErr(w, http.StatusBadRequest, "captcha verification failed")
			return
		}
	} else {
		log.Printf("[AUTH] registration without Turnstile (not configured) user=%s", username)
	}

	// Date of birth (the baby's, not the parent's) — powers the age counter
	// and the start-date countdown in stats. Optional, but validated when set.
	for _, d := range []struct{ label, val string }{{"birth_date", req.BirthDate}, {"start_date", req.StartDate}} {
		if d.val == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d.val); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("zły format %s (chcemy YYYY-MM-DD)", d.label))
			return
		}
	}
	if req.BirthDate != "" {
		if birth, err := time.Parse("2006-01-02", req.BirthDate); err == nil && birth.After(time.Now()) {
			writeErr(w, http.StatusBadRequest, "data urodzenia nie może być z przyszłości 🙂")
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	var userID int
	err = h.DB.QueryRow(
			`INSERT INTO rz_users (username, password_hash, birth_date, start_date)
			 VALUES ($1, $2, NULLIF($3, '')::date, NULLIF($4, '')::date)
			 RETURNING id`,
		username, string(hash), req.BirthDate, req.StartDate,
	).Scan(&userID)
	if err != nil {
		if isPGUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "taki login już istnieje")
			return
		}
		log.Printf("[AUTH] register insert: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Every new baby gets the same starter list (tries: 0).
	if err := db.SeedStarterFoods(h.DB, userID); err != nil {
		log.Printf("[AUTH] register seed foods for user=%d: %v", userID, err)
	}

	log.Printf("[AUTH] type=register user=%s id=%d ip=%s birth=%q start=%q",
		username, userID, clientIP(r), req.BirthDate, req.StartDate)

	// Push notification when it happens.
	if h.Notify != nil {
		h.Notify.UserRegistered(username, req.BirthDate)
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "konto utworzone — możesz się zalogować",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var userID int
	var username, passwordHash string
	err := h.DB.QueryRow(
		`SELECT id, username, password_hash FROM rz_users WHERE username = $1`,
		req.Username,
	).Scan(&userID, &username, &passwordHash)
	if err == sql.ErrNoRows {
		log.Printf("[AUTH] failed login: unknown user %q from %s", req.Username, clientIP(r))
		if h.Notify != nil {
			h.Notify.Send("🔐 Nieudany login", fmt.Sprintf("Nikt taki jak \"%s\" — próba z %s", req.Username, clientIP(r)))
		}
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		log.Printf("[AUTH] login lookup: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		log.Printf("[AUTH] failed login: bad password for %q from %s", username, clientIP(r))
		if h.Notify != nil {
			h.Notify.Send("🔐 Nieudany login", fmt.Sprintf("Złe hasło dla \"%s\" — próba z %s", req.Username, clientIP(r)))
		}
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := middleware.GenerateToken(h.Cfg, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("[AUTH] login user=%s id=%d from %s", username, userID, clientIP(r))
	if h.Notify != nil {
		h.Notify.Send("📱 Login Rozszerzify", fmt.Sprintf("Zalogowano: %s z %s", username, clientIP(r)))
	}
	writeJSON(w, http.StatusOK, authResponse{
		Token:    token,
		UserID:   userID,
		Username: username,
	})
}

// clientIP extracts a plain IP:port from RemoteAddr (RealIP middleware
// rewrites it from X-Forwarded-For), keeping only the host part.
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	return host
}