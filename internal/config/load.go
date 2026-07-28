package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads odoo-builder.yaml at path and merges it over Default(). If path
// does not exist, Default() is returned unchanged.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	return cfg, nil
}
