package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	// Set the name field to the TOML table name/key.
	for name, s := range cfg.Sessions {
		s.Name = name
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, err
}

func validateConfig(cfg Config) error {
	return cfg.Validate()
}
