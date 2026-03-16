package persona

import (
	"encoding/json"

	"github.com/mhai-org/term-ai/internal/db"
)

type Persona struct {
	Name         string
	SystemPrompt string
	Tools        []string // tool IDs the agent may invoke
}

func SetPersona(d *db.Database, name, prompt string, tools []string) error {
	if tools == nil {
		tools = []string{}
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return err
	}
	_, err = d.Conn.Exec(
		"INSERT OR REPLACE INTO personas (name, system_prompt, tools) VALUES (?, ?, ?)",
		name, prompt, string(toolsJSON),
	)
	return err
}

func UnsetPersona(d *db.Database, name string) error {
	_, err := d.Conn.Exec("DELETE FROM personas WHERE name = ?", name)
	return err
}

func ListPersonas(d *db.Database) ([]Persona, error) {
	rows, err := d.Conn.Query("SELECT name, system_prompt, tools FROM personas")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var personas []Persona
	for rows.Next() {
		var p Persona
		var toolsJSON string
		if err := rows.Scan(&p.Name, &p.SystemPrompt, &toolsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(toolsJSON), &p.Tools)
		personas = append(personas, p)
	}
	return personas, nil
}

func GetPersona(d *db.Database, name string) (*Persona, error) {
	var p Persona
	var toolsJSON string
	err := d.Conn.QueryRow(
		"SELECT name, system_prompt, tools FROM personas WHERE name = ?", name,
	).Scan(&p.Name, &p.SystemPrompt, &toolsJSON)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(toolsJSON), &p.Tools)
	return &p, nil
}
