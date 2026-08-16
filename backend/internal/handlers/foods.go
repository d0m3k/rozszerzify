package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rozszerzify/internal/config"

	"github.com/go-chi/chi/v5"
)

// Valid rating levels (how the kid reacted to one try):
// 1 = nie smakuje, 2 = zjada, ale niechętnie, 3 = smakuje, 4 = bardzo smakuje
const (
	RatingMin = 1
	RatingMax = 4
	ratingDefault = 3
)

type FoodHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

type Food struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Tries       int        `json:"tries"`
	Target      int        `json:"target"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastTriedAt *time.Time `json:"last_tried_at"`

	// Rating aggregates from the try log
	RatingAvg   float64 `json:"rating_avg"`
	RatingCount int     `json:"rating_count"`
	RatingSum   int     `json:"rating_sum"`
}

type LogEntry struct {
	ID      int       `json:"id"`
	Note    string    `json:"note"`
	Rating  int       `json:"rating"`
	TriedAt time.Time `json:"tried_at"`
}

type rankingEntry struct {
	FoodID      int     `json:"food_id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Tries       int     `json:"tries"`
	RatingCount int     `json:"rating_count"`
	RatingAvg   float64 `json:"rating_avg"`
	RatingSum   int     `json:"rating_sum"`
}

// fetchFood loads one food (with rating aggregates) for the given user.
func (h *FoodHandler) fetchFood(conn *sql.DB, id, userID int) (*Food, error) {
	f := &Food{}
	err := conn.QueryRow(
		`SELECT f.id, f.name, f.category, f.tries, f.target, f.notes, f.created_at, f.updated_at, f.last_tried_at,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT ROUND(AVG(rating)::numeric, 2)::float8 FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT SUM(rating)::int FROM rz_food_log l WHERE l.food_id = f.id), 0)
		 FROM rz_foods f
		 WHERE f.id = $1 AND f.user_id = $2`,
		id, userID,
	).Scan(&f.ID, &f.Name, &f.Category, &f.Tries, &f.Target, &f.Notes, &f.CreatedAt, &f.UpdatedAt, &f.LastTriedAt,
		&f.RatingCount, &f.RatingAvg, &f.RatingSum)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// List returns all foods for the authenticated user.
func (h *FoodHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := contextUserID(r)

	rows, err := h.DB.Query(
		`SELECT f.id, f.name, f.category, f.tries, f.target, f.notes, f.created_at, f.updated_at, f.last_tried_at,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT ROUND(AVG(rating)::numeric, 2)::float8 FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT SUM(rating)::int FROM rz_food_log l WHERE l.food_id = f.id), 0)
		 FROM rz_foods f
		 WHERE f.user_id = $1
		 ORDER BY f.id`,
		uid,
	)
	if err != nil {
		log.Printf("[FOODS] list: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	foods := []Food{}
	for rows.Next() {
		var f Food
		if err := rows.Scan(&f.ID, &f.Name, &f.Category, &f.Tries, &f.Target, &f.Notes, &f.CreatedAt, &f.UpdatedAt, &f.LastTriedAt,
			&f.RatingCount, &f.RatingAvg, &f.RatingSum); err != nil {
			log.Printf("[FOODS] scan: %v", err)
			continue
		}
		foods = append(foods, f)
	}
	writeJSON(w, http.StatusOK, foods)
}

// Get returns a single food.
func (h *FoodHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}
	f, err := h.fetchFood(h.DB, id, contextUserID(r))
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "food not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type createFoodRequest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Notes    string `json:"notes"`
	Target   int    `json:"target"`
}

// Create adds a new food to the list.
func (h *FoodHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if len([]rune(name)) > 120 {
		writeErr(w, http.StatusBadRequest, "name too long")
		return
	}
	category := normalizeCategory(req.Category)
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 500 {
		writeErr(w, http.StatusBadRequest, "notes too long")
		return
	}
	target := req.Target
	if target < 1 {
		target = 15
	}
	if target > 100 {
		target = 100
	}

	var id int
	err := h.DB.QueryRow(
		`INSERT INTO rz_foods (user_id, name, category, notes, target) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, name) DO NOTHING RETURNING id`,
		contextUserID(r), name, category, notes, target,
	).Scan(&id)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusConflict, "already on the list")
		return
	}
	if err != nil {
		log.Printf("[FOODS] create: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	f, err := h.fetchFood(h.DB, id, contextUserID(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

type tryRequest struct {
	Note   string `json:"note"`
	Rating *int   `json:"rating"` // 1..4; absent → 3 (smakuje)
}

// Try records one offering of the food: increments the counter,
// stamps last_tried_at and appends a log entry.
func (h *FoodHandler) Try(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}
	uid := contextUserID(r)

	var req tryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rating := ratingDefault
	if req.Rating != nil {
		rating = *req.Rating
	}
	if rating < RatingMin || rating > RatingMax {
		writeErr(w, http.StatusBadRequest, "rating must be between 1 and 4")
		return
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 300 {
		writeErr(w, http.StatusBadRequest, "note too long")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM rz_foods WHERE id = $1 AND user_id = $2`, id, uid).Scan(&exists); err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "food not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`INSERT INTO rz_food_log (food_id, user_id, note, rating) VALUES ($1, $2, $3, $4)`,
		id, uid, note, rating,
	); err != nil {
		log.Printf("[FOODS] try insert log: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`UPDATE rz_foods SET tries = tries + 1, last_tried_at = NOW(), updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, uid,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	f, err := h.fetchFood(h.DB, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Untry rolls back the most recent try (mistake / double tap).
func (h *FoodHandler) Untry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}
	uid := contextUserID(r)

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var tries int
	if err := tx.QueryRow(`SELECT tries FROM rz_foods WHERE id = $1 AND user_id = $2`, id, uid).Scan(&tries); err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "food not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if tries > 0 {
		if _, err := tx.Exec(
			`DELETE FROM rz_food_log WHERE id = (SELECT id FROM rz_food_log WHERE food_id = $1 ORDER BY id DESC LIMIT 1)`,
			id,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if _, err := tx.Exec(
			`UPDATE rz_foods
			 SET tries = tries - 1,
			     last_tried_at = (SELECT tried_at FROM rz_food_log WHERE food_id = $1 ORDER BY id DESC LIMIT 1),
			     updated_at = NOW()
			 WHERE id = $1 AND user_id = $2`,
			id, uid,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	f, err := h.fetchFood(h.DB, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type updateFoodRequest struct {
	Name     *string `json:"name"`
	Category *string `json:"category"`
	Notes    *string `json:"notes"`
	Target   *int    `json:"target"`
}

// Update edits editable fields of a food.
func (h *FoodHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}
	uid := contextUserID(r)

	var req updateFoodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sets := []string{}
	args := []interface{}{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		sets = append(sets, "name = "+arg(name))
		sets = append(sets, "updated_at = NOW()")
	}
	if req.Category != nil {
		sets = append(sets, "category = "+arg(normalizeCategory(*req.Category)))
		sets = append(sets, "updated_at = NOW()")
	}
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		if len([]rune(notes)) > 500 {
			writeErr(w, http.StatusBadRequest, "notes too long")
			return
		}
		sets = append(sets, "notes = "+arg(notes))
		sets = append(sets, "updated_at = NOW()")
	}
	if req.Target != nil {
		target := *req.Target
		if target < 1 {
			target = 1
		}
		if target > 100 {
			target = 100
		}
		sets = append(sets, "target = "+arg(target))
		sets = append(sets, "updated_at = NOW()")
	}

	if len(sets) == 0 {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}

	args = append(args, id, uid)
	q := "UPDATE rz_foods SET " + strings.Join(sets, ", ") + " WHERE id = $" + strconv.Itoa(len(args)-1) + " AND user_id = $" + strconv.Itoa(len(args))
	if _, err := h.DB.Exec(q, args...); err != nil {
		if isPGUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "already on the list")
			return
		}
		log.Printf("[FOODS] update: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	f, err := h.fetchFood(h.DB, id, uid)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "food not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Delete removes a food and its log entries (cascade).
func (h *FoodHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM rz_foods WHERE id = $1 AND user_id = $2`, id, contextUserID(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "food not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Log lists the try history of a food (newest first).
func (h *FoodHandler) Log(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}

	rows, err := h.DB.Query(
		`SELECT l.id, l.note, l.rating, l.tried_at
		 FROM rz_food_log l
		 JOIN rz_foods f ON f.id = l.food_id
		 WHERE l.food_id = $1 AND f.user_id = $2
		 ORDER BY l.id DESC
		 LIMIT 100`,
		id, contextUserID(r),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	entries := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Note, &e.Rating, &e.TriedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	writeJSON(w, http.StatusOK, entries)
}

// Ranking lists foods ordered by how well the kid received them
// (avg rating desc, then number of tries desc).
func (h *FoodHandler) Ranking(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(
		`SELECT f.id, f.name, f.category, f.tries,
		        COUNT(l.id)::int AS cnt,
		        COALESCE(ROUND(AVG(l.rating)::numeric, 2)::float8, 0) AS avg_r,
		        COALESCE(SUM(l.rating)::int, 0) AS sum_r
		 FROM rz_foods f
		 LEFT JOIN rz_food_log l ON l.food_id = f.id AND l.user_id = f.user_id
		 WHERE f.user_id = $1
		 GROUP BY f.id
		 ORDER BY avg_r DESC, cnt DESC, f.name ASC`,
		contextUserID(r),
	)
	if err != nil {
		log.Printf("[FOODS] ranking: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	ranking := []rankingEntry{}
	for rows.Next() {
		var e rankingEntry
		if err := rows.Scan(&e.FoodID, &e.Name, &e.Category, &e.Tries, &e.RatingCount, &e.RatingAvg, &e.RatingSum); err != nil {
			continue
		}
		ranking = append(ranking, e)
	}
	writeJSON(w, http.StatusOK, ranking)
}

// Stats returns a small overview for the header.
func (h *FoodHandler) Stats(w http.ResponseWriter, r *http.Request) {
	uid := contextUserID(r)

	var foodsTotal, atTarget, triesTotal int
	if err := h.DB.QueryRow(
		`SELECT COUNT(*)::int,
		        COUNT(*) FILTER (WHERE tries >= target)::int,
		        COALESCE(SUM(tries), 0)::int
		 FROM rz_foods WHERE user_id = $1`,
		uid,
	).Scan(&foodsTotal, &atTarget, &triesTotal); err != nil {
		log.Printf("[FOODS] stats: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	var triedToday int
	if err := h.DB.QueryRow(
		`SELECT COUNT(DISTINCT food_id)::int FROM rz_food_log
		 WHERE user_id = $1 AND tried_at >= date_trunc('day', NOW())`,
		uid,
	).Scan(&triedToday); err != nil {
		triedToday = 0
	}

	start := h.Cfg.StartTime()
	days := int(time.Since(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":         h.Cfg.StartDate,
		"days_since_start":   days,
		"foods_total":        foodsTotal,
		"foods_at_target":    atTarget,
		"tries_total":        triesTotal,
		"foods_tried_today":  triedToday,
	})
}

// normalizeCategory lowercases and trims a category, defaulting to "inne".
func normalizeCategory(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	if c == "" {
		return "inne"
	}
	if len([]rune(c)) > 40 {
		c = string([]rune(c)[:40])
	}
	return c
}