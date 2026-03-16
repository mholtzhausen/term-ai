package agent

import (
"encoding/json"

"github.com/mhai-org/term-ai/internal/db"
)

type Agent struct {
	Name         string
	SystemPrompt string
	Tools        []string
}

func SetAgent(d *db.Database, name, prompt string, tools []string) error {
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

func UnsetAgent(d *db.Database, name string) error {
	_, err := d.Conn.Exec("DELETE FROM personas WHERE name = ?", name)
	return err
}

func ListAgents(d *db.Database) ([]Agent, error) {
	rows, err := d.Conn.Query("SELECT name, system_prompt, tools FROM personas")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var toolsJSON string
		if err := rows.Scan(&a.Name, &a.SystemPrompt, &toolsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(toolsJSON), &a.Tools)
		agents = append(agents, a)
	}
	return agents, nil
}

func GetAgent(d *db.Database, name string) (*Agent, error) {
	var a Agent
	var toolsJSON string
	err := d.Conn.QueryRow(
"SELECT name, system_prompt, tools FROM personas WHERE name = ?", name,
).Scan(&a.Name, &a.SystemPrompt, &toolsJSON)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(toolsJSON), &a.Tools)
	return &a, nil
}
