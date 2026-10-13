package db

import (
	"database/sql"
	"log"
)

// PresetCategory is one of the built-in categories installed for every
// account. Parents can rename, reorder and delete them, so this list is only
// a starting point — never re-applied once the user has categories.
type PresetCategory struct {
	Name  string
	Emoji string
	Order int
}

// PresetCategories mirrors the frontend's classic grouping, in the order the
// list view shows them.
var PresetCategories = []PresetCategory{
	{"alergeny", "⚠️", 10},
	{"warzywa", "🥕", 20},
	{"owoce", "🍎", 30},
	{"kasze i zboża", "🌾", 40},
	{"mięso i ryby", "🍗", 50},
	{"nabiał", "🥛", 60},
	{"inne", "✨", 70},
}

// customCategoryOrder is the sort_order given to categories a parent creates
// (or that show up in old data): after every preset, alphabetical.
const customCategoryOrder = 500

// EnsureCategories makes the category tables consistent with rz_foods:
//
//  1. every account gets the preset categories it is missing,
//  2. every food's primary category (rz_foods.category) exists as a category,
//  3. every food is linked to its primary category in rz_food_categories.
//
// Idempotent — safe (and cheap) to run on every boot, which is how existing
// installations get migrated.
func EnsureCategories(conn *sql.DB) error {
	for _, c := range PresetCategories {
		if _, err := conn.Exec(
			`INSERT INTO rz_categories (user_id, name, emoji, sort_order)
			 SELECT u.id, $1::text, $2::text, $3::int FROM rz_users u
			 WHERE NOT EXISTS (
			     SELECT 1 FROM rz_categories c WHERE c.user_id = u.id AND c.name = $1::text
			 )`,
			c.Name, c.Emoji, c.Order,
		); err != nil {
			return err
		}
	}

	// Categories that only exist because a food carries them (renamed or
	// hand-made ones from before categories were editable).
	if _, err := conn.Exec(
		`INSERT INTO rz_categories (user_id, name, emoji, sort_order)
		 SELECT DISTINCT f.user_id, f.category, '', $1::int FROM rz_foods f
		 WHERE NOT EXISTS (
		     SELECT 1 FROM rz_categories c WHERE c.user_id = f.user_id AND c.name = f.category
		 )`,
		customCategoryOrder,
	); err != nil {
		return err
	}

	if _, err := conn.Exec(
		`INSERT INTO rz_food_categories (food_id, category_id)
		 SELECT f.id, c.id FROM rz_foods f
		 JOIN rz_categories c ON c.user_id = f.user_id AND c.name = f.category
		 WHERE NOT EXISTS (
		     SELECT 1 FROM rz_food_categories fc WHERE fc.food_id = f.id AND fc.category_id = c.id
		 )`,
	); err != nil {
		return err
	}

	return nil
}

// ensureCategoriesLogged runs EnsureCategories and only reports failures —
// a category hiccup must not stop the server from serving.
func EnsureCategoriesLogged(conn *sql.DB) {
	if err := EnsureCategories(conn); err != nil {
		log.Printf("categories: %v", err)
		return
	}
	log.Println("categories ready")
}
