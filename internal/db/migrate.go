package db

import (
	"embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations
var embedMigrations embed.FS

func Migrate(db *sqlx.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	fmt.Println("🔄 Применяем миграции...")

	if err := goose.Up(db.DB, "migrations"); err != nil {
		return fmt.Errorf("❌ goose migration failed: %w", err)
	}

	fmt.Println("✅ Миграции успешно применены")
	return nil
}
