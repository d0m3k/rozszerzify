package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rozszerzify/internal/config"
	"rozszerzify/internal/middleware"
	"rozszerzify/internal/notify"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB     *sql.DB
	Cfg    *config.Config
	Notify *notify.Notifier
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

	log.Printf("[AUTH] login user=%s id=%d", username, userID)
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