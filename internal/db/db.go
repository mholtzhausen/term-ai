package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

type Database struct {
	Conn *sql.DB
}

func Connect() (*Database, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(home, ".term-ai", "term-ai.db")
	dbDir := filepath.Dir(dbPath)

	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{Conn: db}, nil
}

func (db *Database) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			api_key TEXT NOT NULL,
			api_url TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS personas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			system_prompt TEXT NOT NULL,
			tools TEXT NOT NULL DEFAULT '[]',
			provider_id INTEGER,
			model TEXT,
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		);`,
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			history TEXT NOT NULL,
			platform TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			provider_name TEXT,
			model_name TEXT,
			persona_name TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS prompt_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := db.Conn.Exec(q); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	// Migration: add tools column to pre-existing personas tables.
	// Check if 'tools' column exists before attempting to add it.
	columns, err := db.Conn.Query(`PRAGMA table_info(personas)`)
	if err == nil {
		defer columns.Close()
		found := false
		for columns.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dfltValue interface{}
			if err := columns.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
				if name == "tools" {
					found = true
					break
				}
			}
		}
		if !found {
			_, err := db.Conn.Exec(`ALTER TABLE personas ADD COLUMN tools TEXT NOT NULL DEFAULT '[]'`)
			if err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
	}

	return nil
}
