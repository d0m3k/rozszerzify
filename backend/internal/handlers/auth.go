package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"rozszerzify/internal/config"
	"rozszerzify/internal/middleware"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB  *sql.DB
	Cfg *config.Config
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
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		log.Printf("[AUTH] login lookup: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := middleware.GenerateToken(h.Cfg, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("[AUTH] login user=%s id=%d", username, userID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:    token,
		UserID:   userID,
		Username: username,
	})
}