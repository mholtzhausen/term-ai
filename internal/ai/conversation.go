package ai

import (
	"encoding/json"
	"time"
	"github.com/mhai-org/term-ai/internal/db"
)

type Conversation struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	History      []Message `json:"history"`
	Platform     string    `json:"platform"`
	UpdatedAt    time.Time `json:"updated_at"`
	ProviderName string    `json:"provider_name"`
	ModelName    string    `json:"model_name"`
	PersonaName  string    `json:"persona_name"`
}

func SaveConversation(d *db.Database, c *Conversation) error {
	historyJSON, err := json.Marshal(c.History)
	if err != nil {
		return err
	}

	if c.ID != 0 {
		_, err = d.Conn.Exec(`UPDATE conversations SET title = ?, history = ?, platform = ?, updated_at = CURRENT_TIMESTAMP, provider_name = ?, model_name = ?, persona_name = ? WHERE id = ?`,
			c.Title, string(historyJSON), c.Platform, c.ProviderName, c.ModelName, c.PersonaName, c.ID)
	} else {
		res, err := d.Conn.Exec(`INSERT INTO conversations (title, history, platform, updated_at, provider_name, model_name, persona_name) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)`,
			c.Title, string(historyJSON), c.Platform, c.ProviderName, c.ModelName, c.PersonaName)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		c.ID = int(id)
	}
	return err
}

func GetRecentConversation(d *db.Database, platform string) (*Conversation, error) {
	row := d.Conn.QueryRow(`SELECT id, title, history, platform, updated_at, provider_name, model_name, persona_name FROM conversations WHERE platform = ? ORDER BY updated_at DESC LIMIT 1`, platform)
	
	var c Conversation
	var historyJSON string
	var updatedAtStr string
	err := row.Scan(&c.ID, &c.Title, &historyJSON, &c.Platform, &updatedAtStr, &c.ProviderName, &c.ModelName, &c.PersonaName)
	if err != nil {
		return nil, err
	}
	
	// Try multiple common formats
	formats := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z"}
	for _, f := range formats {
		if t, err := time.Parse(f, updatedAtStr); err == nil {
			c.UpdatedAt = t
			break
		}
	}
	// If parsing fails, it might be RFC3339 or similar depending on driver and how it's inserted. 
	// CURRENT_TIMESTAMP in sqlite is usually YYYY-MM-DD HH:MM:SS
	
	if err := json.Unmarshal([]byte(historyJSON), &c.History); err != nil {
		return nil, err
	}
	
	return &c, nil
}

func ListConversations(d *db.Database) ([]Conversation, error) {
	rows, err := d.Conn.Query(`SELECT id, title, history, platform, updated_at, provider_name, model_name, persona_name FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Conversation
	for rows.Next() {
		var c Conversation
		var historyJSON string
		var updatedAtStr string
		if err := rows.Scan(&c.ID, &c.Title, &historyJSON, &c.Platform, &updatedAtStr, &c.ProviderName, &c.ModelName, &c.PersonaName); err != nil {
			return nil, err
		}
		formats := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z"}
		for _, f := range formats {
			if t, err := time.Parse(f, updatedAtStr); err == nil {
				c.UpdatedAt = t
				break
			}
		}
		if err := json.Unmarshal([]byte(historyJSON), &c.History); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}
