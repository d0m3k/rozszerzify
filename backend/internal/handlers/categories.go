package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// categoryCustomOrder places hand-made categories after the presets.
const categoryCustomOrder = 500

// Category is one grouping of foods, owned by an account.
type Category struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Emoji     string `json:"emoji"`
	SortOrder int    `json:"sort_order"`
	FoodCount int    `json:"food_count"`
}

type CategoryHandler struct {
	DB *sql.DB
}

// List returns the account's categories in display order.
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(
		`SELECT c.id, c.name, c.emoji, c.sort_order,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_categories fc WHERE fc.category_id = c.id), 0)
		 FROM rz_categories c
		 WHERE c.user_id = $1
		 ORDER BY c.sort_order, c.name`,
		contextUserID(r),
	)
	if err != nil {
		log.Printf("[CATEGORIES] list: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Emoji, &c.SortOrder, &c.FoodCount); err != nil {
			continue
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

type categoryRequest struct {
	Name      *string `json:"name"`
	Emoji     *string `json:"emoji"`
	SortOrder *int    `json:"sort_order"`
}

// Create adds a category (the parent types a new one in the food form).
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	name := normalizeCategory(*req.Name)
	emoji := normalizeEmoji(req.Emoji)
	order := categoryCustomOrder
	if req.SortOrder != nil {
		order = clampSortOrder(*req.SortOrder)
	}

	var id int
	err := h.DB.QueryRow(
		`INSERT INTO rz_categories (user_id, name, emoji, sort_order) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, name) DO NOTHING RETURNING id`,
		contextUserID(r), name, emoji, order,
	).Scan(&id)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusConflict, "category already exists")
		return
	}
	if err != nil {
		log.Printf("[CATEGORIES] create: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, Category{ID: id, Name: name, Emoji: emoji, SortOrder: order})
}

// Update renames a category, changes its emoji or moves it in the list.
// Renaming also repoints the foods whose primary category it was.
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid category id")
		return
	}
	uid := contextUserID(r)

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == nil && req.Emoji == nil && req.SortOrder == nil {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var oldName string
	if err := tx.QueryRow(`SELECT name FROM rz_categories WHERE id = $1 AND user_id = $2`, id, uid).Scan(&oldName); err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "category not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	sets := []string{}
	args := []interface{}{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if req.Name != nil {
		name := normalizeCategory(*req.Name)
		if name == oldName {
			// nothing to do, but keep the request valid
		} else {
			sets = append(sets, "name = "+arg(name))
		}
	}
	if req.Emoji != nil {
		sets = append(sets, "emoji = "+arg(normalizeEmoji(req.Emoji)))
	}
	if req.SortOrder != nil {
		sets = append(sets, "sort_order = "+arg(clampSortOrder(*req.SortOrder)))
	}

	if len(sets) > 0 {
		args = append(args, id, uid)
		if _, err := tx.Exec(
			"UPDATE rz_categories SET "+strings.Join(sets, ", ")+
				" WHERE id = $"+strconv.Itoa(len(args)-1)+" AND user_id = $"+strconv.Itoa(len(args)),
			args...,
		); err != nil {
			if isPGUniqueViolation(err) {
				writeErr(w, http.StatusConflict, "category already exists")
				return
			}
			log.Printf("[CATEGORIES] update: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	// The primary-category column mirrors the category name.
	if req.Name != nil && normalizeCategory(*req.Name) != oldName {
		if _, err := tx.Exec(
			`UPDATE rz_foods SET category = $1, updated_at = NOW() WHERE user_id = $2 AND category = $3`,
			normalizeCategory(*req.Name), uid, oldName,
		); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	var c Category
	if err := h.DB.QueryRow(
		`SELECT c.id, c.name, c.emoji, c.sort_order,
		        COALESCE((SELECT COUNT(*)::int FROM rz_food_categories fc WHERE fc.category_id = c.id), 0)
		 FROM rz_categories c WHERE c.id = $1 AND c.user_id = $2`,
		id, uid,
	).Scan(&c.ID, &c.Name, &c.Emoji, &c.SortOrder, &c.FoodCount); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// Delete removes a category. Foods that were only in it fall back to their
// next category, or to "inne" when that was their last one.
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid category id")
		return
	}
	uid := contextUserID(r)

	tx, err := h.DB.Begin()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRow(`SELECT name FROM rz_categories WHERE id = $1 AND user_id = $2`, id, uid).Scan(&name); err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "category not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Foods whose *primary* category is going away: repoint them to another
	// category they are already in, else "inne" (created on demand below).
	if _, err := tx.Exec(
		`UPDATE rz_foods f
		 SET category = COALESCE((
		         SELECT c2.name FROM rz_food_categories fc2
		         JOIN rz_categories c2 ON c2.id = fc2.category_id
		         WHERE fc2.food_id = f.id AND fc2.category_id <> $1
		         ORDER BY c2.sort_order, c2.name LIMIT 1
		     ), 'inne'),
		     updated_at = NOW()
		 WHERE f.user_id = $2 AND f.category = $3`,
		id, uid, name,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := tx.Exec(`DELETE FROM rz_categories WHERE id = $1 AND user_id = $2`, id, uid); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Make sure every food's (possibly new) primary category is a real
	// category and is linked. Runs *after* the delete: a food whose last
	// category was removed falls back to "inne", which this recreates.
	if _, err := tx.Exec(
		`INSERT INTO rz_categories (user_id, name, emoji, sort_order)
		 SELECT DISTINCT f.user_id, f.category, '', $1::int FROM rz_foods f
		 WHERE f.user_id = $2
		   AND NOT EXISTS (SELECT 1 FROM rz_categories c WHERE c.user_id = f.user_id AND c.name = f.category)`,
		categoryCustomOrder, uid,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO rz_food_categories (food_id, category_id)
		 SELECT f.id, c.id FROM rz_foods f
		 JOIN rz_categories c ON c.user_id = f.user_id AND c.name = f.category
		 WHERE f.user_id = $1
		   AND NOT EXISTS (SELECT 1 FROM rz_food_categories fc WHERE fc.food_id = f.id AND fc.category_id = c.id)`,
		uid,
	); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// normalizeEmoji keeps an optional category emoji short and harmless.
func normalizeEmoji(e *string) string {
	if e == nil {
		return ""
	}
	v := strings.TrimSpace(*e)
	r := []rune(v)
	if len(r) > 4 {
		v = string(r[:4])
	}
	return v
}

func clampSortOrder(v int) int {
	if v < 0 {
		return 0
	}
	if v > 9999 {
		return 9999
	}
	return v
}

// normalizeCategoryList cleans a set of category names: lowercase, trimmed,
// deduplicated, primary (first) kept in front. Empty input → ["inne"].
func normalizeCategoryList(names []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, n := range names {
		c := strings.ToLower(strings.TrimSpace(n))
		if len([]rune(c)) > 40 {
			c = string([]rune(c)[:40])
		}
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return []string{"inne"}
	}
	return out
}

// ensureCategoryID returns the id of a category, creating it when the parent
// typed a name that does not exist yet.
func ensureCategoryID(tx *sql.Tx, uid int, name string) (int, error) {
	var id int
	err := tx.QueryRow(`SELECT id FROM rz_categories WHERE user_id = $1 AND name = $2`, uid, name).Scan(&id)
	if err == sql.ErrNoRows {
		err = tx.QueryRow(
			`INSERT INTO rz_categories (user_id, name, sort_order) VALUES ($1, $2, $3) RETURNING id`,
			uid, name, categoryCustomOrder,
		).Scan(&id)
	}
	return id, err
}

// setFoodCategories replaces a food's category set; names[0] becomes the
// primary category (rz_foods.category). Reports whether the food exists.
func setFoodCategories(tx *sql.Tx, uid, foodID int, names []string) (bool, error) {
	res, err := tx.Exec(
		`UPDATE rz_foods SET category = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		names[0], foodID, uid,
	)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return false, nil
	}
	if _, err := tx.Exec(`DELETE FROM rz_food_categories WHERE food_id = $1`, foodID); err != nil {
		return false, err
	}
	for _, n := range names {
		id, err := ensureCategoryID(tx, uid, n)
		if err != nil {
			return false, err
		}
		if _, err := tx.Exec(
			`INSERT INTO rz_food_categories (food_id, category_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			foodID, id,
		); err != nil {
			return false, err
		}
	}
	return true, nil
}
