package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
)

// A meal (danie/posiłek) is a reusable set of existing foods. Eating it
// records one try for the meal itself and one try for every ingredient, with
// the ingredient tries pointing back at the meal event.
type MealHandler struct {
	DB *sql.DB
}

type MealItem struct {
	FoodID   int    `json:"food_id"`
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	Category string `json:"category"`
}

type Meal struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Emoji       string     `json:"emoji"`
	Notes       string     `json:"notes"`
	Tries       int        `json:"tries"`
	Target      int        `json:"target"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastTriedAt *time.Time `json:"last_tried_at"`

	RatingAvg   float64 `json:"rating_avg"`
	RatingCount int     `json:"rating_count"`
	RatingSum   int     `json:"rating_sum"`
	LastNote    string  `json:"last_note"`
	LastRating  int     `json:"last_rating"`

	RecentRatings []int `json:"recent_ratings"`

	// The foods this meal consists of, in serving order.
	Items []MealItem `json:"items"`
}

type mealRequest struct {
	Name   string `json:"name"`
	Emoji  string `json:"emoji"`
	Notes  string `json:"notes"`
	Target int    `json:"target"`
	Items  []int  `json:"items"`
}

type updateMealRequest struct {
	Name   *string `json:"name"`
	Emoji  *string `json:"emoji"`
	Notes  *string `json:"notes"`
	Target *int    `json:"target"`
	Items  *[]int  `json:"items"`
}

// fetchMeal loads one meal and its composition for the given user.
func (h *MealHandler) fetchMeal(conn *sql.DB, id, userID int) (*Meal, error) {
	m := &Meal{}
	var recent pq.Int64Array
	err := conn.QueryRow(
		`SELECT m.id, m.name, m.emoji, m.notes, m.tries, m.target, m.created_at, m.updated_at, m.last_tried_at,
		        COALESCE((SELECT COUNT(*)::int FROM rz_meal_log l WHERE l.meal_id = m.id), 0),
		        COALESCE((SELECT ROUND(AVG(rating)::numeric, 2)::float8 FROM rz_meal_log l WHERE l.meal_id = m.id), 0),
		        COALESCE((SELECT SUM(rating)::int FROM rz_meal_log l WHERE l.meal_id = m.id), 0),
		        COALESCE((SELECT l.note FROM rz_meal_log l WHERE l.meal_id = m.id ORDER BY l.tried_at DESC, l.id DESC LIMIT 1), ''),
		        COALESCE((SELECT l.rating FROM rz_meal_log l WHERE l.meal_id = m.id ORDER BY l.tried_at DESC, l.id DESC LIMIT 1), 0),
		        COALESCE((SELECT ARRAY(SELECT l.rating FROM rz_meal_log l WHERE l.meal_id = m.id ORDER BY l.tried_at DESC, l.id DESC LIMIT 5)), ARRAY[]::int[])
		 FROM rz_meals m
		 WHERE m.id = $1 AND m.user_id = $2`,
		id, userID,
	).Scan(&m.ID, &m.Name, &m.Emoji, &m.Notes, &m.Tries, &m.Target, &m.CreatedAt, &m.UpdatedAt, &m.LastTriedAt,
		&m.RatingCount, &m.RatingAvg, &m.RatingSum, &m.LastNote, &m.LastRating, &recent)
	if err != nil {
		return nil, err
	}
	m.RecentRatings = intSlice(recent)
	m.Items = []MealItem{}

	rows, err := conn.Query(
		`SELECT mi.food_id, f.name, f.emoji, f.category
		 FROM rz_meal_items mi
		 JOIN rz_foods f ON f.id = mi.food_id
		 WHERE mi.meal_id = $1
		 ORDER BY mi.position, f.name`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it MealItem
		if err := rows.Scan(&it.FoodID, &it.Name, &it.Emoji, &it.Category); err != nil {
			return nil, err
		}
		m.Items = append(m.Items, it)
	}
	return m, rows.Err()
}

// List returns all meals (with their composition) for the authenticated user.
func (h *MealHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := contextUserID(r)

	rows, err := h.DB.Query(`SELECT id FROM rz_meals WHERE user_id = $1 ORDER BY name`, uid)
	if err != nil {
		log.Printf("[MEALS] list: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	meals := []Meal{}
	for _, id := range ids {
		m, err := h.fetchMeal(h.DB, id, uid)
		if err != nil {
			continue
		}
		meals = append(meals, *m)
	}
	writeJSON(w, http.StatusOK, meals)
}

// Get returns a single meal with its composition.
func (h *MealHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	m, err := h.fetchMeal(h.DB, id, contextUserID(r))
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "meal not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// normalizeMealFields validates the shared name/emoji/notes/target fields.
func normalizeMealFields(name, emoji, notes *string, target *int) (string, string, string, int, string) {
	n := strings.ToLower(strings.TrimSpace(*name))
	if n == "" {
		return "", "", "", 0, "name is required"
	}
	if len([]rune(n)) > 120 {
		return "", "", "", 0, "name too long"
	}
	nt := strings.TrimSpace(*notes)
	if len([]rune(nt)) > 500 {
		return "", "", "", 0, "notes too long"
	}
	t := *target
	if t < 1 {
		t = 15
	}
	if t > 100 {
		t = 100
	}
	return n, normalizeEmoji(emoji), nt, t, ""
}

// setMealItems replaces the composition of a meal. Every food must belong to
// the user. Reports false when one of the ids is not the user's food.
func setMealItems(tx *sql.Tx, uid, mealID int, ids []int) (bool, error) {
	var count int
	if err := tx.QueryRow(
		`SELECT COUNT(*)::int FROM rz_foods WHERE user_id = $1 AND id = ANY($2)`,
		uid, pq.Array(ids),
	).Scan(&count); err != nil {
		return false, err
	}
	if count != len(ids) {
		return false, nil
	}
	if _, err := tx.Exec(`DELETE FROM rz_meal_items WHERE meal_id = $1`, mealID); err != nil {
		return false, err
	}
	for i, foodID := range ids {
		if _, err := tx.Exec(
			`INSERT INTO rz_meal_items (meal_id, food_id, position) VALUES ($1, $2, $3)`,
			mealID, foodID, i,
		); err != nil {
			return false, err
		}
	}
	return true, nil
}

// mealItems normalizes a client-supplied composition (dedupe, keep order).
func mealItems(ids []int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 1 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// Create adds a meal built from existing foods.
func (h *MealHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req mealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name, emoji, notes, target, msg := normalizeMealFields(&req.Name, &req.Emoji, &req.Notes, &req.Target)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	items := mealItems(req.Items)
	if len(items) == 0 {
		writeErr(w, http.StatusBadRequest, "a meal needs at least one food")
		return
	}

	uid := contextUserID(r)
	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRow(
		`INSERT INTO rz_meals (user_id, name, emoji, notes, target) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, name) DO NOTHING RETURNING id`,
		uid, name, emoji, notes, target,
	).Scan(&id)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusConflict, "already on the list")
		return
	}
	if err != nil {
		log.Printf("[MEALS] create: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if ok, err := setMealItems(tx, uid, id, items); err != nil {
		log.Printf("[MEALS] create items: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	} else if !ok {
		writeErr(w, http.StatusBadRequest, "items must be existing foods")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	m, err := h.fetchMeal(h.DB, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// Update edits a meal's settings and/or composition.
func (h *MealHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	uid := contextUserID(r)

	var req updateMealRequest
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
		if len([]rune(name)) > 120 {
			writeErr(w, http.StatusBadRequest, "name too long")
			return
		}
		sets = append(sets, "name = "+arg(name))
	}
	if req.Emoji != nil {
		sets = append(sets, "emoji = "+arg(normalizeEmoji(req.Emoji)))
	}
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		if len([]rune(notes)) > 500 {
			writeErr(w, http.StatusBadRequest, "notes too long")
			return
		}
		sets = append(sets, "notes = "+arg(notes))
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
	}

	var items []int
	if req.Items != nil {
		items = mealItems(*req.Items)
		if len(items) == 0 {
			writeErr(w, http.StatusBadRequest, "a meal needs at least one food")
			return
		}
	}

	if len(sets) == 0 && req.Items == nil {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at = NOW()")
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	if len(sets) > 0 {
		args = append(args, id, uid)
		res, err := tx.Exec(
			"UPDATE rz_meals SET "+strings.Join(sets, ", ")+
				" WHERE id = $"+strconv.Itoa(len(args)-1)+" AND user_id = $"+strconv.Itoa(len(args)),
			args...,
		)
		if err != nil {
			if isPGUniqueViolation(err) {
				writeErr(w, http.StatusConflict, "already on the list")
				return
			}
			log.Printf("[MEALS] update: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if n, err := res.RowsAffected(); err == nil && n == 0 {
			writeErr(w, http.StatusNotFound, "meal not found")
			return
		}
	}

	if req.Items != nil {
		ok, err := setMealItems(tx, uid, id, items)
		if err != nil {
			log.Printf("[MEALS] update items: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !ok {
			writeErr(w, http.StatusBadRequest, "items must be existing foods")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	m, err := h.fetchMeal(h.DB, id, uid)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "meal not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// parseMealTry validates the rating, note and timestamp shared by try/log edits.
func parseMealTry(req tryRequest) (int, string, *time.Time, string) {
	rating := ratingDefault
	if req.Rating != nil {
		rating = *req.Rating
	}
	if rating < RatingMin || rating > RatingMax {
		return 0, "", nil, "rating must be between 1 and 4"
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 300 {
		return 0, "", nil, "note too long"
	}
	var triedAt *time.Time
	if strings.TrimSpace(req.TriedAt) != "" {
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(req.TriedAt))
		if err != nil {
			return 0, "", nil, "tried_at must be an RFC3339 timestamp"
		}
		if ts.After(time.Now().Add(triedAtFutureSkew)) {
			return 0, "", nil, "tried_at cannot be in the future"
		}
		if ts.Before(minTriedAt) {
			return 0, "", nil, "tried_at is too far in the past"
		}
		triedAt = &ts
	}
	return rating, note, triedAt, ""
}

// tryCheck loads the meal and returns whether it exists for the user.
func mealExists(tx *sql.Tx, id, uid int) (bool, error) {
	var one int
	err := tx.QueryRow(`SELECT 1 FROM rz_meals WHERE id = $1 AND user_id = $2`, id, uid).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// Try records one eaten meal: the meal counter goes up and every ingredient
// food gets its own try linked to this meal event.
func (h *MealHandler) Try(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	uid := contextUserID(r)

	var req tryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rating, note, triedAt, msg := parseMealTry(req)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	ok, err := mealExists(tx, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "meal not found")
		return
	}

	var mealName string
	if err := tx.QueryRow(`SELECT name FROM rz_meals WHERE id = $1`, id).Scan(&mealName); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	var mealLogID int
	if err := tx.QueryRow(
		`INSERT INTO rz_meal_log (meal_id, user_id, note, rating, tried_at)
		 VALUES ($1, $2, $3, $4, COALESCE($5::timestamptz, NOW())) RETURNING id`,
		id, uid, note, rating, triedAt,
	).Scan(&mealLogID); err != nil {
		log.Printf("[MEALS] try insert log: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// One try per ingredient, marked with the meal it came from.
	if _, err := tx.Exec(
		`INSERT INTO rz_food_log (food_id, user_id, note, rating, tried_at, meal_log_id, meal_name)
		 SELECT mi.food_id, $2, $3, $4, COALESCE($5::timestamptz, NOW()), $1, $6
		 FROM rz_meal_items mi
		 WHERE mi.meal_id = $7`,
		mealLogID, uid, note, rating, triedAt, mealName, id,
	); err != nil {
		log.Printf("[MEALS] try insert food logs: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := tx.Exec(
		`UPDATE rz_foods f
		 SET tries = f.tries + 1,
		     last_tried_at = GREATEST(f.last_tried_at, COALESCE($3::timestamptz, NOW())),
		     updated_at = NOW()
		 WHERE f.user_id = $2 AND f.id IN (SELECT food_id FROM rz_meal_items WHERE meal_id = $1)`,
		id, uid, triedAt,
	); err != nil {
		log.Printf("[MEALS] try bump foods: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`UPDATE rz_meals
		 SET tries = tries + 1,
		     last_tried_at = GREATEST(last_tried_at, COALESCE($3::timestamptz, NOW())),
		     updated_at = NOW()
		 WHERE id = $1 AND user_id = $2`,
		id, uid, triedAt,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	m, err := h.fetchMeal(h.DB, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// deleteMealEvent removes one recorded meal event plus the ingredient tries it
// generated, then fixes both counters/timestamps. Caller owns the transaction.
func deleteMealEvent(tx *sql.Tx, uid, mealID, logID int) error {
	rows, err := tx.Query(
		`SELECT DISTINCT food_id FROM rz_food_log WHERE meal_log_id = $1`,
		logID,
	)
	if err != nil {
		return err
	}
	foodIDs := []int{}
	for rows.Next() {
		var fid int
		if err := rows.Scan(&fid); err != nil {
			rows.Close()
			return err
		}
		foodIDs = append(foodIDs, fid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM rz_food_log WHERE meal_log_id = $1`, logID); err != nil {
		return err
	}
	if len(foodIDs) > 0 {
		if _, err := tx.Exec(
			`UPDATE rz_foods f
			 SET tries = GREATEST(f.tries - 1, 0),
			     last_tried_at = (SELECT MAX(tried_at) FROM rz_food_log l WHERE l.food_id = f.id),
			     updated_at = NOW()
			 WHERE f.user_id = $1 AND f.id = ANY($2)`,
			uid, pq.Array(foodIDs),
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM rz_meal_log WHERE id = $1`, logID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE rz_meals
		 SET tries = GREATEST(tries - 1, 0),
		     last_tried_at = (SELECT MAX(tried_at) FROM rz_meal_log WHERE meal_id = $1),
		     updated_at = NOW()
		 WHERE id = $1 AND user_id = $2`,
		mealID, uid,
	); err != nil {
		return err
	}
	return nil
}

// Untry rolls back the most recent meal event (and its ingredient tries).
func (h *MealHandler) Untry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	uid := contextUserID(r)

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	ok, err := mealExists(tx, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "meal not found")
		return
	}

	var logID int
	err = tx.QueryRow(
		`SELECT id FROM rz_meal_log WHERE meal_id = $1 AND user_id = $2 ORDER BY id DESC LIMIT 1`,
		id, uid,
	).Scan(&logID)
	if err != nil && err != sql.ErrNoRows {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err == nil {
		if err := deleteMealEvent(tx, uid, id, logID); err != nil {
			log.Printf("[MEALS] untry: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	m, err := h.fetchMeal(h.DB, id, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Delete removes a meal definition. Past food tries stay in the food history
// (they keep the denormalized meal name); the meal's own history goes away.
func (h *MealHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	res, err := h.DB.Exec(`DELETE FROM rz_meals WHERE id = $1 AND user_id = $2`, id, contextUserID(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, http.StatusNotFound, "meal not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Log lists the try history of a meal (newest first).
func (h *MealHandler) Log(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	rows, err := h.DB.Query(
		`SELECT l.id, l.note, l.rating, l.tried_at
		 FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.meal_id = $1 AND m.user_id = $2
		 ORDER BY l.tried_at DESC, l.id DESC
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

type updateMealLogRequest struct {
	Rating  *int    `json:"rating"`
	Note    *string `json:"note"`
	TriedAt string  `json:"tried_at"`
}

// UpdateLog edits one meal event and mirrors the change onto the ingredient
// tries it generated, keeping the two histories consistent.
func (h *MealHandler) UpdateLog(w http.ResponseWriter, r *http.Request) {
	mealID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || mealID < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
		return
	}
	logID, err := strconv.Atoi(chi.URLParam(r, "logId"))
	if err != nil || logID < 1 {
		writeErr(w, http.StatusBadRequest, "invalid log id")
		return
	}
	uid := contextUserID(r)

	var req updateMealLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var triedAt *time.Time
	if strings.TrimSpace(req.TriedAt) != "" {
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(req.TriedAt))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "tried_at must be an RFC3339 timestamp")
			return
		}
		if ts.After(time.Now().Add(triedAtFutureSkew)) {
			writeErr(w, http.StatusBadRequest, "tried_at cannot be in the future")
			return
		}
		if ts.Before(minTriedAt) {
			writeErr(w, http.StatusBadRequest, "tried_at is too far in the past")
			return
		}
		triedAt = &ts
	}

	sets := []string{}
	args := []interface{}{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if req.Rating != nil {
		if *req.Rating < RatingMin || *req.Rating > RatingMax {
			writeErr(w, http.StatusBadRequest, "rating must be between 1 and 4")
			return
		}
		sets = append(sets, "rating = "+arg(*req.Rating))
	}
	if req.Note != nil {
		note := strings.TrimSpace(*req.Note)
		if len([]rune(note)) > 300 {
			writeErr(w, http.StatusBadRequest, "note too long")
			return
		}
		sets = append(sets, "note = "+arg(note))
	}
	if triedAt != nil {
		sets = append(sets, "tried_at = "+arg(*triedAt))
	}
	if len(sets) == 0 {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var mealIDOfEntry int
	err = tx.QueryRow(
		`SELECT l.meal_id FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.id = $1 AND m.user_id = $2`,
		logID, uid,
	).Scan(&mealIDOfEntry)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "entry not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if mealIDOfEntry != mealID {
		writeErr(w, http.StatusBadRequest, "entry does not belong to this meal")
		return
	}

	args = append(args, logID)
	if _, err := tx.Exec(
		"UPDATE rz_meal_log SET "+strings.Join(sets, ", ")+" WHERE id = $"+strconv.Itoa(len(args)),
		args...,
	); err != nil {
		log.Printf("[MEALS] update log: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Mirror onto the ingredient tries of this event.
	mirror := []string{}
	margs := []interface{}{}
	marg := func(v interface{}) string {
		margs = append(margs, v)
		return "$" + strconv.Itoa(len(margs))
	}
	if req.Rating != nil {
		mirror = append(mirror, "rating = "+marg(*req.Rating))
	}
	if req.Note != nil {
		mirror = append(mirror, "note = "+marg(strings.TrimSpace(*req.Note)))
	}
	if triedAt != nil {
		mirror = append(mirror, "tried_at = "+marg(*triedAt))
	}
	margs = append(margs, logID)
	if _, err := tx.Exec(
		"UPDATE rz_food_log SET "+strings.Join(mirror, ", ")+" WHERE meal_log_id = $"+strconv.Itoa(len(margs)),
		margs...,
	); err != nil {
		log.Printf("[MEALS] update linked food logs: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if triedAt != nil {
		if _, err := tx.Exec(
			`UPDATE rz_foods f
			 SET last_tried_at = (SELECT MAX(tried_at) FROM rz_food_log l WHERE l.food_id = f.id),
			     updated_at = NOW()
			 WHERE f.user_id = $1
			   AND f.id IN (SELECT DISTINCT food_id FROM rz_food_log WHERE meal_log_id = $2)`,
			uid, logID,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	m, err := h.fetchMeal(h.DB, mealID, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// DeleteLog removes one meal event together with the ingredient tries it
// generated, so counters never drift.
func (h *MealHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	mealID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || mealID < 1 {
		writeErr(w, http.StatusBadRequest, "invalid meal id")
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

	var mealIDOfEntry int
	err = tx.QueryRow(
		`SELECT l.meal_id FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.id = $1 AND m.user_id = $2`,
		logID, uid,
	).Scan(&mealIDOfEntry)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "entry not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if mealIDOfEntry != mealID {
		writeErr(w, http.StatusBadRequest, "entry does not belong to this meal")
		return
	}

	if err := deleteMealEvent(tx, uid, mealID, logID); err != nil {
		log.Printf("[MEALS] delete log: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	m, err := h.fetchMeal(h.DB, mealID, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}
