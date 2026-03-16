package ui

import "github.com/mhai-org/term-ai/internal/db"

const promptHistoryLimit = 100

// savePromptHistory inserts a prompt into the database and prunes entries
// beyond the most recent promptHistoryLimit rows.
func savePromptHistory(d *db.Database, prompt string) error {
	if _, err := d.Conn.Exec(
		`INSERT INTO prompt_history (prompt) VALUES (?)`, prompt,
	); err != nil {
		return err
	}
	// Prune oldest entries so the table never exceeds the limit.
	_, err := d.Conn.Exec(`
		DELETE FROM prompt_history
		WHERE id NOT IN (
			SELECT id FROM prompt_history
			ORDER BY id DESC
			LIMIT ?
		)`, promptHistoryLimit)
	return err
}

// loadPromptHistory returns the most recent prompts, newest first.
func loadPromptHistory(d *db.Database) ([]string, error) {
	rows, err := d.Conn.Query(
		`SELECT prompt FROM prompt_history ORDER BY id DESC LIMIT ?`,
		promptHistoryLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prompts []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}
