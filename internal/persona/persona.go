package persona

import (
	"github.com/mhai-org/mhai/internal/db"
)

type Persona struct {
	Name         string
	SystemPrompt string
}

func SetPersona(d *db.Database, name, prompt string) error {
	_, err := d.Conn.Exec("INSERT OR REPLACE INTO personas (name, system_prompt) VALUES (?, ?)", name, prompt)
	return err
}

func UnsetPersona(d *db.Database, name string) error {
	_, err := d.Conn.Exec("DELETE FROM personas WHERE name = ?", name)
	return err
}

func ListPersonas(d *db.Database) ([]Persona, error) {
	rows, err := d.Conn.Query("SELECT name, system_prompt FROM personas")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var personas []Persona
	for rows.Next() {
		var p Persona
		if err := rows.Scan(&p.Name, &p.SystemPrompt); err != nil {
			return nil, err
		}
		personas = append(personas, p)
	}
	return personas, nil
}

func GetPersona(d *db.Database, name string) (*Persona, error) {
	var p Persona
	err := d.Conn.QueryRow("SELECT name, system_prompt FROM personas WHERE name = ?", name).Scan(&p.Name, &p.SystemPrompt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
