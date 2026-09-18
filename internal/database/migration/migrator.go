package migration

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// Migration represents a single database migration with an Up (apply) and Down (rollback) SQL.
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// Migrator manages and executes database migrations.
type Migrator struct {
	db         *sql.DB
	migrations []Migration
	logger     *slog.Logger
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(db *sql.DB, logger *slog.Logger) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
	}
}

// AddMigration registers a migration to the migrator.
func (m *Migrator) AddMigration(migration Migration) {
	m.migrations = append(m.migrations, migration)
}

// Init creates the schema_migrations tracking table if it doesn't exist.
func (m *Migrator) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		dirty      BOOLEAN NOT NULL DEFAULT FALSE,
		applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`
	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	m.logger.Info("Migration tracker table ready")
	return nil
}

// currentVersion returns the latest applied migration version, or 0 if none.
func (m *Migrator) currentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// Up applies all pending migrations in order.
func (m *Migrator) Up() error {
	if err := m.Init(); err != nil {
		return err
	}

	current, err := m.currentVersion()
	if err != nil {
		return err
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	applied := 0
	for _, mg := range m.migrations {
		if mg.Version <= current {
			continue
		}

		m.logger.Info("Applying migration",
			slog.Int("version", mg.Version),
			slog.String("description", mg.Description),
		)

		tx, err := m.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", mg.Version, err)
		}

		// Mark as dirty before executing
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, dirty, applied_at) VALUES ($1, TRUE, $2)", mg.Version, time.Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: failed to mark as dirty: %w", mg.Version, err)
		}

		// Execute migration SQL
		if _, err := tx.Exec(mg.Up); err != nil {
			tx.Rollback()
			// Clean up dirty record on failure
			m.db.Exec("DELETE FROM schema_migrations WHERE version = $1", mg.Version)
			return fmt.Errorf("migration %d (%s) failed: %w", mg.Version, mg.Description, err)
		}

		// Mark as clean (not dirty)
		if _, err := tx.Exec("UPDATE schema_migrations SET dirty = FALSE WHERE version = $1", mg.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: failed to mark as clean: %w", mg.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: failed to commit: %w", mg.Version, err)
		}

		m.logger.Info("Migration applied successfully",
			slog.Int("version", mg.Version),
			slog.String("description", mg.Description),
		)
		applied++
	}

	if applied == 0 {
		m.logger.Info("No pending migrations. Database is up to date.",
			slog.Int("current_version", current),
		)
	} else {
		newVersion, _ := m.currentVersion()
		m.logger.Info("All migrations applied",
			slog.Int("total_applied", applied),
			slog.Int("current_version", newVersion),
		)
	}

	return nil
}

// Down rolls back the last applied migration.
func (m *Migrator) Down() error {
	if err := m.Init(); err != nil {
		return err
	}

	current, err := m.currentVersion()
	if err != nil {
		return err
	}

	if current == 0 {
		m.logger.Info("No migrations to rollback")
		return nil
	}

	// Find the migration matching the current version
	var target *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == current {
			target = &m.migrations[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("migration version %d not found in registered migrations", current)
	}

	m.logger.Info("Rolling back migration",
		slog.Int("version", target.Version),
		slog.String("description", target.Description),
	)

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin rollback transaction: %w", err)
	}

	if _, err := tx.Exec(target.Down); err != nil {
		tx.Rollback()
		return fmt.Errorf("rollback of migration %d (%s) failed: %w", target.Version, target.Description, err)
	}

	if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", target.Version); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record %d: %w", target.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rollback commit failed: %w", err)
	}

	m.logger.Info("Migration rolled back successfully",
		slog.Int("version", target.Version),
		slog.String("description", target.Description),
	)

	return nil
}

// Status prints the current migration status.
func (m *Migrator) Status() error {
	if err := m.Init(); err != nil {
		return err
	}

	current, err := m.currentVersion()
	if err != nil {
		return err
	}

	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	fmt.Println("┌─────────┬────────────┬──────────────────────────────────────────┐")
	fmt.Println("│ Version │   Status   │ Description                              │")
	fmt.Println("├─────────┼────────────┼──────────────────────────────────────────┤")

	for _, mg := range m.migrations {
		status := "pending"
		if mg.Version <= current {
			status = "applied"
		}
		fmt.Printf("│ %7d │ %-10s │ %-40s │\n", mg.Version, status, mg.Description)
	}

	fmt.Println("└─────────┴────────────┴──────────────────────────────────────────┘")
	fmt.Printf("\nCurrent version: %d\n", current)

	return nil
}

// Reset rolls back ALL migrations (use with caution).
func (m *Migrator) Reset() error {
	if err := m.Init(); err != nil {
		return err
	}

	current, err := m.currentVersion()
	if err != nil {
		return err
	}

	if current == 0 {
		m.logger.Info("No migrations to reset")
		return nil
	}

	// Sort descending for rollback order
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version > m.migrations[j].Version
	})

	for _, mg := range m.migrations {
		if mg.Version > current {
			continue
		}
		if err := m.Down(); err != nil {
			return err
		}
	}

	m.logger.Info("All migrations rolled back")
	return nil
}
