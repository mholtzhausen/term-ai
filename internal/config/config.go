package config

import (
	"github.com/mhai-org/mhai/internal/db"
	"github.com/mhai-org/mhai/internal/security"
)

type Provider struct {
	Name   string
	ApiKey string
	ApiUrl string
}

func SetProvider(d *db.Database, name, key, url string) error {
	encryptedKey, err := security.Encrypt(key)
	if err != nil {
		return err
	}
	_, err = d.Conn.Exec("INSERT OR REPLACE INTO providers (name, api_key, api_url) VALUES (?, ?, ?)", name, encryptedKey, url)
	return err
}

func ListProviders(d *db.Database) ([]Provider, error) {
	rows, err := d.Conn.Query("SELECT name, api_key, api_url FROM providers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		var p Provider
		var encryptedKey string
		if err := rows.Scan(&p.Name, &encryptedKey, &p.ApiUrl); err != nil {
			return nil, err
		}
		key, err := security.Decrypt(encryptedKey)
		if err != nil {
			p.ApiKey = "[ERROR DECRYPTING]"
		} else {
			p.ApiKey = key
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func GetProvider(d *db.Database, name string) (*Provider, error) {
	var p Provider
	var encryptedKey string
	err := d.Conn.QueryRow("SELECT name, api_key, api_url FROM providers WHERE name = ?", name).Scan(&p.Name, &encryptedKey, &p.ApiUrl)
	if err != nil {
		return nil, err
	}
	key, err := security.Decrypt(encryptedKey)
	if err != nil {
		return nil, err
	}
	p.ApiKey = key
	return &p, nil
}
