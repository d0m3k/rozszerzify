package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// Meal graphs are small by design: a danie can contain another danie ("zupka
// na rosole"), but never itself, directly or through other meals. These caps
// also stop accidental fan-out (one bowl = 200 tries). They are enforced on
// every composition edit and re-checked for the whole account, so a parent
// can't outgrow the limits just because one of its children grew.
const (
	maxMealDepth      = 3  // root + up to 3 nesting levels
	maxMealComponents = 20 // direct children: foods + sub-meals
	maxMealFoods      = 40 // distinct products after full expansion
	maxMealSubMeals   = 24 // distinct component meals after full expansion
	mealWalkDepth     = 8  // defensive stop for traversal (corrupt data)
)

// compositionError marks a user-fixable problem with a meal's composition.
type compositionError struct{ msg string }

func (e compositionError) Error() string { return e.msg }

type MealItem struct {
	FoodID   int    `json:"food_id"`
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	Category string `json:"category"`
}

// SubMeal is another meal used as a component of this one.
type SubMeal struct {
	MealID    int    `json:"meal_id"`
	Name      string `json:"name"`
	Emoji     string `json:"emoji"`
	FoodCount int    `json:"food_count"`
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

	// Other meals used as components ("zupka na rosole" → rosół), in order.
	SubMeals []SubMeal `json:"sub_meals"`

	// Size of the fully expanded composition: what one +1 really records.
	ExpandedFoods int `json:"expanded_foods"`
	ExpandedMeals int `json:"expanded_meals"`
}

type mealRequest struct {
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	Notes    string `json:"notes"`
	Target   int    `json:"target"`
	Items    []int  `json:"items"`
	SubMeals []int  `json:"sub_meals"`
}

type updateMealRequest struct {
	Name     *string `json:"name"`
	Emoji    *string `json:"emoji"`
	Notes    *string `json:"notes"`
	Target   *int    `json:"target"`
	Items    *[]int  `json:"items"`
	SubMeals *[]int  `json:"sub_meals"`
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	m.SubMeals = []SubMeal{}
	subRows, err := conn.Query(
		`SELECT s.sub_meal_id, sm.name, sm.emoji,
		        (SELECT COUNT(*)::int FROM rz_meal_items mi WHERE mi.meal_id = sm.id)
		 FROM rz_meal_submeals s
		 JOIN rz_meals sm ON sm.id = s.sub_meal_id
		 WHERE s.meal_id = $1
		 ORDER BY s.position, sm.name`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer subRows.Close()
	for subRows.Next() {
		var sm SubMeal
		if err := subRows.Scan(&sm.MealID, &sm.Name, &sm.Emoji, &sm.FoodCount); err != nil {
			return nil, err
		}
		m.SubMeals = append(m.SubMeals, sm)
	}
	if err := subRows.Err(); err != nil {
		return nil, err
	}

	m.ExpandedFoods, m.ExpandedMeals, err = mealExpansion(conn, id)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// mealExpansion counts what one try of this meal expands to: distinct products
// and distinct component meals (the root itself not included). The traversal
// is path-guarded so it terminates even on corrupt data.
func mealExpansion(conn *sql.DB, mealID int) (foods, meals int, err error) {
	err = conn.QueryRow(
		`WITH RECURSIVE walk(cur_id, depth, path) AS (
		     SELECT m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.id = $1
		     UNION ALL
		     SELECT s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
		     FROM walk w
		     JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
		     WHERE w.depth < $2 AND NOT (s.sub_meal_id = ANY(w.path))
		 )
		 SELECT (SELECT COUNT(DISTINCT mi.food_id)::int
		         FROM walk w JOIN rz_meal_items mi ON mi.meal_id = w.cur_id),
		        (SELECT COUNT(*)::int FROM walk WHERE depth > 0)`,
		mealID, mealWalkDepth,
	).Scan(&foods, &meals)
	return foods, meals, err
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

// lockMealGraph serializes composition writes per account. Without it two
// concurrent requests could each pass the "does it already contain me?"
// check and together create a cycle.
func lockMealGraph(tx *sql.Tx, uid int) error {
	_, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, int64(uid))
	return err
}

// mealComponents loads the current composition of a meal (for partial updates).
func mealComponents(tx *sql.Tx, mealID int) (foodIDs, subMealIDs []int, err error) {
	rows, err := tx.Query(`SELECT food_id FROM rz_meal_items WHERE meal_id = $1 ORDER BY position, food_id`, mealID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		foodIDs = append(foodIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	rows, err = tx.Query(`SELECT sub_meal_id FROM rz_meal_submeals WHERE meal_id = $1 ORDER BY position, sub_meal_id`, mealID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		subMealIDs = append(subMealIDs, id)
	}
	return foodIDs, subMealIDs, rows.Err()
}

// setMealComposition replaces the whole composition of a meal and validates
// it: ownership, self-reference, cycles, depth and fan-out. The caller owns
// the transaction and must hold lockMealGraph.
func setMealComposition(tx *sql.Tx, uid, mealID int, foods, subMeals []int) error {
	if len(foods)+len(subMeals) == 0 {
		return compositionError{"a meal needs at least one food or meal"}
	}
	if len(foods)+len(subMeals) > maxMealComponents {
		return compositionError{fmt.Sprintf("too many components (max %d)", maxMealComponents)}
	}

	if len(foods) > 0 {
		var count int
		if err := tx.QueryRow(
			`SELECT COUNT(*)::int FROM rz_foods WHERE user_id = $1 AND id = ANY($2)`,
			uid, pq.Array(foods),
		).Scan(&count); err != nil {
			return err
		}
		if count != len(foods) {
			return compositionError{"items must be existing foods"}
		}
	}

	if len(subMeals) > 0 {
		var count int
		if err := tx.QueryRow(
			`SELECT COUNT(*)::int FROM rz_meals WHERE user_id = $1 AND id = ANY($2)`,
			uid, pq.Array(subMeals),
		).Scan(&count); err != nil {
			return err
		}
		if count != len(subMeals) {
			return compositionError{"sub-meals must be meals of this account"}
		}
	}

	// Adding X to this meal is only legal when X does not already contain it
	// (directly or transitively); otherwise we would close a loop.
	for _, subID := range subMeals {
		if subID == mealID {
			return compositionError{"a meal cannot contain itself"}
		}
		var name string
		err := tx.QueryRow(
			`WITH RECURSIVE down(id, depth, path) AS (
			     SELECT $1::int, 0, ARRAY[$1::int]
			     UNION ALL
			     SELECT s.sub_meal_id, d.depth + 1, d.path || s.sub_meal_id
			     FROM down d JOIN rz_meal_submeals s ON s.meal_id = d.id
			     WHERE d.depth < $2 AND NOT (s.sub_meal_id = ANY(d.path))
			 )
			 SELECT m.name FROM down d JOIN rz_meals m ON m.id = d.id
			 WHERE d.id = $3 AND d.depth > 0 LIMIT 1`,
			subID, mealWalkDepth, mealID,
		).Scan(&name)
		if err == nil {
			return compositionError{fmt.Sprintf("%q already contains this meal — that would create a cycle", name)}
		}
		if err != sql.ErrNoRows {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM rz_meal_items WHERE meal_id = $1`, mealID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rz_meal_submeals WHERE meal_id = $1`, mealID); err != nil {
		return err
	}
	for i, foodID := range foods {
		if _, err := tx.Exec(
			`INSERT INTO rz_meal_items (meal_id, food_id, position) VALUES ($1, $2, $3)`,
			mealID, foodID, i,
		); err != nil {
			return err
		}
	}
	for i, subID := range subMeals {
		if _, err := tx.Exec(
			`INSERT INTO rz_meal_submeals (meal_id, sub_meal_id, position) VALUES ($1, $2, $3)`,
			mealID, subID, i,
		); err != nil {
			return err
		}
	}

	return validateMealGraph(tx, uid)
}

// validateMealGraph re-checks the whole account DAG after an edit. This catches
// limits violated indirectly: meal B is fine on its own, but growing it can
// push its parent over the depth or fan-out cap.
func validateMealGraph(tx *sql.Tx, uid int) error {
	var foreign int
	if err := tx.QueryRow(
		`SELECT COUNT(*)::int FROM rz_meal_submeals s
		 JOIN rz_meals p ON p.id = s.meal_id
		 JOIN rz_meals c ON c.id = s.sub_meal_id
		 WHERE (p.user_id = $1 OR c.user_id = $1) AND p.user_id <> c.user_id`,
		uid,
	).Scan(&foreign); err != nil {
		return err
	}
	if foreign > 0 {
		return compositionError{"a meal can only contain meals from the same account"}
	}

	walk := `WITH RECURSIVE walk(root_id, cur_id, depth, path) AS (
	             SELECT m.id, m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.user_id = $1
	             UNION ALL
	             SELECT w.root_id, s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
	             FROM walk w JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
	             WHERE w.depth < $2 AND NOT (s.sub_meal_id = ANY(w.path))
	         )`

	var cycles int
	if err := tx.QueryRow(
		walk+` SELECT COUNT(*)::int
		 FROM rz_meal_submeals s JOIN walk w ON w.cur_id = s.meal_id
		 WHERE s.sub_meal_id = ANY(w.path)`,
		uid, mealWalkDepth,
	).Scan(&cycles); err != nil {
		return err
	}
	if cycles > 0 {
		return compositionError{"composition would create a cycle"}
	}

	rows, err := tx.Query(
		walk+` SELECT w.root_id, m.name,
		        MAX(w.depth)::int,
		        (SELECT COUNT(*)::int FROM walk x WHERE x.root_id = w.root_id AND x.depth > 0),
		        (SELECT COUNT(DISTINCT mi.food_id)::int FROM walk x
		         JOIN rz_meal_items mi ON mi.meal_id = x.cur_id WHERE x.root_id = w.root_id)
		 FROM walk w JOIN rz_meals m ON m.id = w.root_id
		 GROUP BY w.root_id, m.name`,
		uid, mealWalkDepth,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			rootID, depth, meals, foods int
			name                        string
		)
		if err := rows.Scan(&rootID, &name, &depth, &meals, &foods); err != nil {
			return err
		}
		if depth > maxMealDepth {
			return compositionError{fmt.Sprintf("%q would nest meals too deeply (max %d levels)", name, maxMealDepth)}
		}
		if meals > maxMealSubMeals {
			return compositionError{fmt.Sprintf("%q would contain too many meals (max %d)", name, maxMealSubMeals)}
		}
		if foods > maxMealFoods {
			return compositionError{fmt.Sprintf("%q would contain too many foods (max %d)", name, maxMealFoods)}
		}
	}
	return rows.Err()
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
	subMeals := mealItems(req.SubMeals)
	if len(items)+len(subMeals) == 0 {
		writeErr(w, http.StatusBadRequest, "a meal needs at least one food or meal")
		return
	}

	uid := contextUserID(r)
	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	if err := lockMealGraph(tx, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

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

	if err := setMealComposition(tx, uid, id, items, subMeals); err != nil {
		writeCompositionErr(w, err, "create composition")
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

	var items, subMeals []int
	compositionChanged := req.Items != nil || req.SubMeals != nil
	if req.Items != nil {
		items = mealItems(*req.Items)
	}
	if req.SubMeals != nil {
		subMeals = mealItems(*req.SubMeals)
	}

	if len(sets) == 0 && !compositionChanged {
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

	if err := lockMealGraph(tx, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

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

	if compositionChanged {
		// Partial updates keep the half that was not sent.
		if req.Items == nil || req.SubMeals == nil {
			curFoods, curSubs, err := mealComponents(tx, id)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "internal error")
				return
			}
			if req.Items == nil {
				items = curFoods
			}
			if req.SubMeals == nil {
				subMeals = curSubs
			}
		}
		if err := setMealComposition(tx, uid, id, items, subMeals); err != nil {
			writeCompositionErr(w, err, "update composition")
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

// writeCompositionErr maps composition validation failures to 400s.
func writeCompositionErr(w http.ResponseWriter, err error, what string) {
	var ce compositionError
	if errors.As(err, &ce) {
		writeErr(w, http.StatusBadRequest, ce.msg)
		return
	}
	log.Printf("[MEALS] %s: %v", what, err)
	writeErr(w, http.StatusInternalServerError, "internal error")
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

	if err := lockMealGraph(tx, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

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

	// One timestamp for the whole serving: the parent event, the component
	// meal events and every product try must line up.
	ts := time.Now()
	if triedAt != nil {
		ts = *triedAt
	}

	var mealLogID int
	if err := tx.QueryRow(
		`INSERT INTO rz_meal_log (meal_id, user_id, note, rating, tried_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		id, uid, note, rating, ts,
	).Scan(&mealLogID); err != nil {
		log.Printf("[MEALS] try insert log: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Meals used as components also count as tried, in the same serving. The
	// recursion is path-guarded (corrupt data can't loop) and DISTINCT: a meal
	// reachable through two paths still gets exactly one try.
	if _, err := tx.Exec(
		`WITH RECURSIVE walk(cur_id, depth, path) AS (
		     SELECT m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.id = $3 AND m.user_id = $2
		     UNION ALL
		     SELECT s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
		     FROM walk w
		     JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
		     JOIN rz_meals m ON m.id = s.sub_meal_id AND m.user_id = $2
		     WHERE w.depth < $4 AND NOT (s.sub_meal_id = ANY(w.path))
		 )
		 INSERT INTO rz_meal_log (meal_id, user_id, note, rating, tried_at, parent_log_id)
		 SELECT DISTINCT cur_id, $2::int, $5::text, $6::int, $7::timestamptz, $1::int FROM walk WHERE depth > 0`,
		mealLogID, uid, id, mealWalkDepth, note, rating, ts,
	); err != nil {
		log.Printf("[MEALS] try insert component logs: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// One try per distinct product across the whole expanded composition — a
	// product that is both a direct item and an ingredient of a component meal
	// is recorded once. All rows stay attached to the root event, so undo and
	// history edits keep working exactly as before.
	if _, err := tx.Exec(
		`WITH RECURSIVE walk(cur_id, depth, path) AS (
		     SELECT m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.id = $3 AND m.user_id = $2
		     UNION ALL
		     SELECT s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
		     FROM walk w
		     JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
		     JOIN rz_meals m ON m.id = s.sub_meal_id AND m.user_id = $2
		     WHERE w.depth < $4 AND NOT (s.sub_meal_id = ANY(w.path))
		 )
		 INSERT INTO rz_food_log (food_id, user_id, note, rating, tried_at, meal_log_id, meal_name)
		 SELECT DISTINCT mi.food_id, $2::int, $5::text, $6::int, $7::timestamptz, $1::int, $8::text
		 FROM walk w JOIN rz_meal_items mi ON mi.meal_id = w.cur_id`,
		mealLogID, uid, id, mealWalkDepth, note, rating, ts, mealName,
	); err != nil {
		log.Printf("[MEALS] try insert food logs: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`WITH RECURSIVE walk(cur_id, depth, path) AS (
		     SELECT m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.id = $1 AND m.user_id = $2
		     UNION ALL
		     SELECT s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
		     FROM walk w
		     JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
		     JOIN rz_meals m ON m.id = s.sub_meal_id AND m.user_id = $2
		     WHERE w.depth < $3 AND NOT (s.sub_meal_id = ANY(w.path))
		 )
		 UPDATE rz_foods f
		 SET tries = f.tries + 1,
		     last_tried_at = GREATEST(f.last_tried_at, $4::timestamptz),
		     updated_at = NOW()
		 WHERE f.user_id = $2
		   AND f.id IN (SELECT DISTINCT mi.food_id FROM walk w
		                JOIN rz_meal_items mi ON mi.meal_id = w.cur_id)`,
		id, uid, mealWalkDepth, ts,
	); err != nil {
		log.Printf("[MEALS] try bump foods: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(
		`WITH RECURSIVE walk(cur_id, depth, path) AS (
		     SELECT m.id, 0, ARRAY[m.id] FROM rz_meals m WHERE m.id = $1 AND m.user_id = $2
		     UNION ALL
		     SELECT s.sub_meal_id, w.depth + 1, w.path || s.sub_meal_id
		     FROM walk w
		     JOIN rz_meal_submeals s ON s.meal_id = w.cur_id
		     JOIN rz_meals m ON m.id = s.sub_meal_id AND m.user_id = $2
		     WHERE w.depth < $3 AND NOT (s.sub_meal_id = ANY(w.path))
		 )
		 UPDATE rz_meals m
		 SET tries = m.tries + 1,
		     last_tried_at = GREATEST(m.last_tried_at, $4::timestamptz),
		     updated_at = NOW()
		 WHERE m.user_id = $2 AND m.id IN (SELECT DISTINCT cur_id FROM walk)`,
		id, uid, mealWalkDepth, ts,
	); err != nil {
		log.Printf("[MEALS] try bump meals: %v", err)
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

// deleteMealEvent removes one recorded meal event, every component-meal try it
// created and all the product tries that came from it, then fixes the
// counters/timestamps. Caller owns the transaction.
func deleteMealEvent(tx *sql.Tx, uid, logID int) error {
	var mealID int
	if err := tx.QueryRow(
		`SELECT l.meal_id FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.id = $1 AND m.user_id = $2`,
		logID, uid,
	).Scan(&mealID); err != nil {
		return err
	}

	// Meals that were only tried because they were components of this event.
	mealIDs := []int{mealID}
	rows, err := tx.Query(
		`SELECT l.meal_id FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.parent_log_id = $1 AND m.user_id = $2`,
		logID, uid,
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		mealIDs = append(mealIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = tx.Query(
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
	if _, err := tx.Exec(`DELETE FROM rz_meal_log WHERE parent_log_id = $1`, logID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rz_meal_log WHERE id = $1`, logID); err != nil {
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

	// Each of these meals got exactly +1 from this event, so -1 is exact even
	// if the composition changed since; the timestamp is rebuilt from what is
	// left in their history.
	if _, err := tx.Exec(
		`UPDATE rz_meals m
		 SET tries = GREATEST(m.tries - 1, 0),
		     last_tried_at = (SELECT MAX(tried_at) FROM rz_meal_log l WHERE l.meal_id = m.id),
		     updated_at = NOW()
		 WHERE m.user_id = $1 AND m.id = ANY($2)`,
		uid, pq.Array(mealIDs),
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
		`SELECT id FROM rz_meal_log
		 WHERE meal_id = $1 AND user_id = $2 AND parent_log_id IS NULL
		 ORDER BY id DESC LIMIT 1`,
		id, uid,
	).Scan(&logID)
	if err != nil && err != sql.ErrNoRows {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err == nil {
		if err := deleteMealEvent(tx, uid, logID); err != nil {
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
		`SELECT l.id, l.note, l.rating, l.tried_at, l.parent_log_id,
		        COALESCE((SELECT pm.name FROM rz_meal_log pl
		                  JOIN rz_meals pm ON pm.id = pl.meal_id
		                  WHERE pl.id = l.parent_log_id), '')
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
		if err := rows.Scan(&e.ID, &e.Note, &e.Rating, &e.TriedAt, &e.ParentLogID, &e.ParentMeal); err != nil {
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
	var parentLogID *int
	err = tx.QueryRow(
		`SELECT l.meal_id, l.parent_log_id FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.id = $1 AND m.user_id = $2`,
		logID, uid,
	).Scan(&mealIDOfEntry, &parentLogID)
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

	// A component-meal event only exists inside its parent serving: editing it
	// changes just this entry.
	if parentLogID == nil {
		// Mirror onto the product tries of this event.
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
		where := " WHERE meal_log_id = $" + strconv.Itoa(len(margs))
		if _, err := tx.Exec(
			"UPDATE rz_food_log SET "+strings.Join(mirror, ", ")+where,
			margs...,
		); err != nil {
			log.Printf("[MEALS] update linked food logs: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		// ... and onto the component-meal events of this serving.
		if _, err := tx.Exec(
			"UPDATE rz_meal_log SET "+strings.Join(mirror, ", ")+" WHERE parent_log_id = $"+strconv.Itoa(len(margs)),
			margs...,
		); err != nil {
			log.Printf("[MEALS] update component meal logs: %v", err)
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
	var parentLogID *int
	var parentMealName string
	err = tx.QueryRow(
		`SELECT l.meal_id, l.parent_log_id,
		        COALESCE((SELECT pm.name FROM rz_meal_log pl
		                  JOIN rz_meals pm ON pm.id = pl.meal_id
		                  WHERE pl.id = l.parent_log_id), '')
		 FROM rz_meal_log l
		 JOIN rz_meals m ON m.id = l.meal_id
		 WHERE l.id = $1 AND m.user_id = $2`,
		logID, uid,
	).Scan(&mealIDOfEntry, &parentLogID, &parentMealName)
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
	// A component meal was eaten as part of a bigger serving — removing it
	// alone would desynchronise the products of that serving.
	if parentLogID != nil {
		writeErr(w, http.StatusConflict, fmt.Sprintf(
			"this try is part of %q — remove that serving instead", parentMealName))
		return
	}

	if err := deleteMealEvent(tx, uid, logID); err != nil {
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
