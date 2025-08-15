package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrationsAndSeed(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (id SERIAL PRIMARY KEY, name TEXT UNIQUE NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		b, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		// Split by ; and run each statement (simple runner sufficient for our SQL)
		stmts := strings.Split(string(b), ";")
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s failed: %w", name, err)
			}
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(name) VALUES($1)`, name); err != nil {
			return err
		}
	}

	// Seed admin and default invite if not exists
	var adminCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_admin=true`).Scan(&adminCount); err != nil {
		return err
	}
	if adminCount == 0 {
		passwordHash := hashPassword("admin")
		pinHash := hashPassword("0000")
		_, err := db.ExecContext(ctx, `INSERT INTO users (username, display_name, password_hash, pin_hash, public_key, is_admin) VALUES ($1,$2,$3,$4,$5,true)`,
			"admin", "Administrator", passwordHash, pinHash, "")
		if err != nil {
			return err
		}
	}
	var inviteCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invites`).Scan(&inviteCount); err != nil {
		return err
	}
	if inviteCount == 0 {
		_, err := db.ExecContext(ctx, `INSERT INTO invites (code, created_by, max_uses, uses, active) VALUES ($1, NULL, $2, 0, true)`, "DEFAULT-INVITE-0001", 1000)
		if err != nil {
			return err
		}
	}
	return nil
}

func hashPassword(p string) string {
	// Argon2id parameters
	salt := []byte("static-seed-salt-change-me")	// In production replace with per-user random salt stored alongside hash
	key := argon2.IDKey([]byte(p), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("argon2id$v=19$m=65536,t=1,p=4$%x$%x", salt, key)
}