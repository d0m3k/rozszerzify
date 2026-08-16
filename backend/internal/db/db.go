package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

//go:embed migration.sql
var migrationFS embed.FS

// Connect opens a PostgreSQL connection and runs migrations.
func Connect(databaseURL string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Println("database connected")

	if err := runMigrations(conn); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return conn, nil
}

func runMigrations(conn *sql.DB) error {
	migration, err := migrationFS.ReadFile("migration.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	if _, err := conn.Exec(string(migration)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	log.Println("migrations applied")
	return nil
}