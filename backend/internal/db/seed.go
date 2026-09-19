package db

import (
	"database/sql"
	"log"
)

// StarterFoods is installed on every new account (first boot, the -new-user
// CLI mode and self-service registration). Every entry starts at 0 tries —
// the parent fills in what the kid has already tried.
var StarterFoods = []struct{ Name, Category string }{
	// alergeny (wprowadzać pojedynczo, po jednym na kilka dni,
	// w małych ilościach — najważniejsza kategoria)
	{"jajko (całe)", "alergeny"},
	{"orzeszki ziemne", "alergeny"},
	{"migdały", "alergeny"},
	{"orzechy laskowe", "alergeny"},
	{"sezam (tahini)", "alergeny"},
	{"mleko krowie (do picia)", "alergeny"},
	{"gluten (kasza manna)", "alergeny"},
	{"soja (tofu)", "alergeny"},
	{"krewetki", "alergeny"},
	// warzywa
	{"marchewka", "warzywa"},
	{"ziemniak", "warzywa"},
	{"brokuł", "warzywa"},
	{"kalafior", "warzywa"},
	{"dynia", "warzywa"},
	{"cukinia", "warzywa"},
	{"batat", "warzywa"},
	{"groszek zielony", "warzywa"},
	{"burak", "warzywa"},
	{"pietruszka", "warzywa"},
	{"awokado", "warzywa"},
	// owoce
	{"jabłko", "owoce"},
	{"gruszka", "owoce"},
	{"banan", "owoce"},
	{"morela", "owoce"},
	{"brzoskwinia", "owoce"},
	{"śliwka", "owoce"},
	{"malina", "owoce"},
	{"borówka", "owoce"},
	{"mango", "owoce"},
	// kasze i zboża
	{"kasza jaglana", "kasze i zboża"},
	{"kaszka ryżowa", "kasze i zboża"},
	{"kaszka kukurydziana", "kasze i zboża"},
	{"płatki owsiane", "kasze i zboża"},
	// mięso i ryby
	{"indyk", "mięso i ryby"},
	{"kurczak", "mięso i ryby"},
	{"cielęcina", "mięso i ryby"},
	{"łosoś", "mięso i ryby"},
	{"dorsz", "mięso i ryby"},
	{"żółtko jaja", "mięso i ryby"},
	// nabiał
	{"jogurt naturalny", "nabiał"},
	{"twarożek", "nabiał"},
	// inne
	{"oliwa z oliwek", "inne"},
	{"olej rzepakowy", "inne"},
	{"siemię lniane", "inne"},
}

// SeedStarterFoods installs the starter food list for a user that has no
// foods yet. No-op when the user already has anything on their list.
func SeedStarterFoods(conn *sql.DB, uid int) error {
	var foods int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM rz_foods WHERE user_id = $1`, uid).Scan(&foods); err != nil {
		return err
	}
	if foods == 0 {
		for _, f := range StarterFoods {
			if _, err := conn.Exec(
				`INSERT INTO rz_foods (user_id, name, category) VALUES ($1, $2, $3)
				 ON CONFLICT (user_id, name) DO NOTHING`,
				uid, f.Name, f.Category,
			); err != nil {
				log.Printf("  seed food %q: %v", f.Name, err)
			}
		}
		log.Printf("  %d starter foods added", len(StarterFoods))
	}
	return nil
}
