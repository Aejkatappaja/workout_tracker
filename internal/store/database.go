package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Open() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db:open %w", err)
	}

	configurePool(db)
	slog.Default().Info("connected to database")

	return db, nil
}

// configurePool bounds the connection pool. The database/sql default is unlimited
// open connections, which under load opens one Postgres backend per in-flight
// query and exhausts the server's max_connections ("too many clients"). Capping
// it makes requests queue on the pool instead; idle/old connections are recycled.
func configurePool(db *sql.DB) {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(time.Hour)
}

func MigrateFS(db *sql.DB, migrationFS fs.FS, dir string) error {
	goose.SetBaseFS(migrationFS)
	defer func() {
		goose.SetBaseFS(nil)
	}()

	err := goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("migrate %w", err)
	}

	err = goose.Up(db, dir)
	if err != nil {
		return fmt.Errorf("goose up %w", err)
	}
	return nil
}

func Migrate(db *sql.DB, dir string) error {
	err := goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("migrate %w", err)
	}

	err = goose.Up(db, dir)
	if err != nil {
		return fmt.Errorf("goose up %w", err)
	}
	return nil
}
