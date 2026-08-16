package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"rozszerzify/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[JSON] encode error: %v", err)
	}
}

// writeErr writes a small JSON error body, keeping the {"error":...} contract.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// isPGUniqueViolation checks for PostgreSQL unique violation (code 23505).
func isPGUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "23505")
}

// contextUserID extracts the authenticated user id injected by middleware.
func contextUserID(r *http.Request) int {
	uid, _ := r.Context().Value(middleware.ContextUserID).(int)
	return uid
}