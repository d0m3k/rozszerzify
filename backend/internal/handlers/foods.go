package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rozszerzify/internal/config"
	"rozszerzify/internal/notify"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
)

// Valid rating levels (how the kid reacted to one try):
// 1 = nie smakuje, 2 = zjada, ale niechętnie, 3 = smakuje, 4 = bardzo smakuje
const (
	RatingMin = 1
	RatingMax = 4
	ratingDefault = 3
)

type FoodHandler struct {
	DB     *sql.DB
	Cfg    *config.Config
	Notify *notify.Notifier
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

	// Note of the most recent try (per-serving note, more important than the
	// food-level note for quick glances)
	// Note of the most recent try (per-serving note, more important than the
	// food-level note for quick glances)
	LastNote string `json:"last_note"`

	// Rating of the most recent try (0 when the food was never tried), plus a
	// small trace of the last few ratings for the "minki" row.
	LastRating    int   `json:"last_rating"`
	RecentRatings []int `json:"recent_ratings"`
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
	var recent pq.Int64Array
	err := conn.QueryRow(
		`SELECT f.id, f.name, f.category, f.tries, f.target, f.notes, f.created_at, f.updated_at, f.last_tried_at,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT ROUND(AVG(rating)::numeric, 2)::float8 FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT SUM(rating)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT l.note FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), ''),
		        COALESCE((SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), 0),
		        COALESCE((SELECT ARRAY(SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 5)), ARRAY[]::int[])
		 FROM rz_foods f
		 WHERE f.id = $1 AND f.user_id = $2`,
		id, userID,
	).Scan(&f.ID, &f.Name, &f.Category, &f.Tries, &f.Target, &f.Notes, &f.CreatedAt, &f.UpdatedAt, &f.LastTriedAt,
		&f.RatingCount, &f.RatingAvg, &f.RatingSum, &f.LastNote, &f.LastRating, &recent)
	if err != nil {
		log.Printf("[FOODS] fetch id=%d: %v", id, err)
		return nil, err
	}
	f.RecentRatings = intSlice(recent)
	return f, nil
}

// List returns all foods for the authenticated user.
func (h *FoodHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := contextUserID(r)

	rows, err := h.DB.Query(
		`SELECT f.id, f.name, f.category, f.tries, f.target, f.notes, f.created_at, f.updated_at, f.last_tried_at,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT ROUND(AVG(rating)::numeric, 2)::float8 FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT SUM(rating)::int FROM rz_food_log l WHERE l.food_id = f.id), 0),
		        COALESCE((SELECT l.note FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), ''),
		        COALESCE((SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), 0),
		        COALESCE((SELECT ARRAY(SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 5)), ARRAY[]::int[])
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
		var recent pq.Int64Array
		if err := rows.Scan(&f.ID, &f.Name, &f.Category, &f.Tries, &f.Target, &f.Notes, &f.CreatedAt, &f.UpdatedAt, &f.LastTriedAt,
			&f.RatingCount, &f.RatingAvg, &f.RatingSum, &f.LastNote, &f.LastRating, &recent); err != nil {
			log.Printf("[FOODS] scan: %v", err)
			continue
		}
		f.RecentRatings = intSlice(recent)
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

	name := strings.ToLower(strings.TrimSpace(req.Name))
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

	var prevTries, prevLastRating int
	if err := tx.QueryRow(
		`SELECT f.tries, COALESCE((SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), 0)
		 FROM rz_foods f WHERE f.id = $1 AND f.user_id = $2`,
		id, uid,
	).Scan(&prevTries, &prevLastRating); err == sql.ErrNoRows {
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

	// Pushover notifications for the milestone moments: first try,
	// 15-try target reached, and — most importantly — a green last try that
	// marks the food as OK without needing all 15 attempts.
	if h.Notify != nil && h.Notify.Enabled() {
		reaction := ratingEmoji(rating)
		wasOK := prevTries >= f.Target || prevLastRating >= 3
		nowOK := f.Tries >= f.Target || f.LastRating >= 3
		switch {
		case prevTries == 0:
			h.Notify.Send("🍽️ Nowe jedzenie", fmt.Sprintf("%s — pierwszy raz! (%s)", f.Name, reaction))
		case f.Tries >= f.Target && prevTries < f.Target:
			h.Notify.Send("🎉 Przetestowane!", fmt.Sprintf("%s ma %d prób (%s) — wiesz już, czy smakuje.", f.Name, f.Tries, reaction))
		case !wasOK && nowOK:
			h.Notify.Send("💚 Nowe OK!", fmt.Sprintf("%s — zjadł przy ostatniej próbie (%s), nie trzeba już czekać na 15 prób!", f.Name, reaction))
		}
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
		name := strings.ToLower(strings.TrimSpace(*req.Name))
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

// DeleteLog removes one specific try entry (✕ in the history list) and
// fixes the food counter + last_tried_at accordingly.
func (h *FoodHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	foodID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || foodID < 1 {
		writeErr(w, http.StatusBadRequest, "invalid food id")
		return
	}
	logID, err := strconv.Atoi(chi.URLParam(r, "logId"))
	if err != nil || logID < 1 {
		writeErr(w, http.StatusBadRequest, "invalid log id")
		return
	}
	uid := contextUserID(r)

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	// The entry must belong to the given food AND the food to the user.
	var foodIDOfEntry int
	err = tx.QueryRow(
		`SELECT l.food_id FROM rz_food_log l
		 JOIN rz_foods f ON f.id = l.food_id
		 WHERE l.id = $1 AND f.user_id = $2`,
		logID, uid,
	).Scan(&foodIDOfEntry)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "entry not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if foodIDOfEntry != foodID {
		writeErr(w, http.StatusBadRequest, "entry does not belong to this food")
		return
	}

	if _, err := tx.Exec(`DELETE FROM rz_food_log WHERE id = $1`, logID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`UPDATE rz_foods
		 SET tries = GREATEST(tries - 1, 0),
		     last_tried_at = (SELECT tried_at FROM rz_food_log WHERE food_id = $1 ORDER BY id DESC LIMIT 1),
		     updated_at = NOW()
		 WHERE id = $1 AND user_id = $2`,
		foodID, uid,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	f, err := h.fetchFood(h.DB, foodID, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Ranking lists foods ordered by how well the kid received them
// (avg rating desc, then number of tries desc).
// intSlice converts a pq int array into []int.
func intSlice(a pq.Int64Array) []int {
	out := make([]int, len(a))
	for i, v := range a {
		out[i] = int(v)
	}
	return out
}

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

	// "OK" foods: reached the 15-try target OR the most recent try was green
	// (rating >= 3 — the kid ate it), which means we can stop focusing on it.
	var foodsOK int
	if err := h.DB.QueryRow(
		`SELECT COUNT(*)::int FROM rz_foods f
		 WHERE f.user_id = $1
		   AND (f.tries >= f.target
		        OR COALESCE((SELECT l.rating FROM rz_food_log l WHERE l.food_id = f.id ORDER BY l.id DESC LIMIT 1), 0) >= 3)`,
		uid,
	).Scan(&foodsOK); err != nil {
		foodsOK = 0
	}

	var daysSinceStart, daysUntilStart int
	started := false
	if h.Cfg.StartDate != "" {
		start := h.Cfg.StartTime()
		// The diet may not have started yet (start date in the future): show a
		// countdown instead of a day counter. All math on UTC midnights so the
		// day boundaries are unambiguous.
		y, m, d := time.Now().UTC().Date()
		nowMid := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		sy, sm, sd := start.UTC().Date()
		startMid := time.Date(sy, sm, sd, 0, 0, 0, 0, time.UTC)

		started = !nowMid.Before(startMid)
		if started {
			daysSinceStart = int(nowMid.Sub(startMid).Hours()/24) + 1
		} else {
			daysUntilStart = int(startMid.Sub(nowMid).Hours() / 24)
		}
	}

	// Baby's age (calendar months + leftover days). Only when a birth date
	// is configured (kept out of the public repo).
	ageMonths, ageDays := 0, 0
	if h.Cfg.BirthDate != "" {
		ageMonths, ageDays = ageMonthsDays(h.Cfg.BirthTime(), time.Now())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":       h.Cfg.StartDate,
		"started":          started,
		"days_since_start": daysSinceStart,
		"days_until_start": daysUntilStart,
		"birth_date":       h.Cfg.BirthDate,
		"baby_age_months":  ageMonths,
		"baby_age_days":    ageDays,
		"foods_total":       foodsTotal,
		"foods_ok":          foodsOK,
		"foods_at_target":   atTarget,
		"tries_total":       triesTotal,
		"foods_tried_today": triedToday,
	})
}

// ratingEmoji maps a 1–4 rating to a short emoji for notifications and labels.
func ratingEmoji(r int) string {
	switch r {
	case 1:
		return "😖"
	case 2:
		return "😐"
	case 3:
		return "🙂"
	case 4:
		return "🤩"
	}
	return "🙂"
}

// ageMonthsDays returns the calendar age as (months, days) between birth and now.
func ageMonthsDays(birth, now time.Time) (int, int) {
	if now.Before(birth) {
		return 0, 0
	}
	months := (now.Year()-birth.Year())*12 + int(now.Month()) - int(birth.Month())
	if now.Day() < birth.Day() {
		months--
	}
	anchor := birth.AddDate(0, months, 0)
	if anchor.After(now) {
		anchor = birth.AddDate(0, months-1, 0)
		months--
	}
	next := anchor.AddDate(0, 1, 0)
	days := int(now.Sub(anchor).Hours() / 24)
	monthLen := int(next.Sub(anchor).Hours() / 24)
	if days >= monthLen {
		months++
		days = 0
	}
	return months, days
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