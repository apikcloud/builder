// Package config parses the optional odoo-builder.yaml file.
package config

// Config is the full odoo-builder.yaml schema. Every field is optional;
// see Default for the zero-configuration behavior.
type Config struct {
	Base        Base              `yaml:"base"`
	Enterprise  Enterprise        `yaml:"enterprise"`
	Addons      Addons            `yaml:"addons"`
	Submodules  Submodules        `yaml:"submodules"`
	Build       Build             `yaml:"build"`
	Cache       Cache             `yaml:"cache"`
	Image       Image             `yaml:"image"`
	Labels      map[string]string `yaml:"labels"`
	Environment map[string]string `yaml:"environment"`
}

type Base struct {
	Version string `yaml:"version"`
	Release string `yaml:"release"`
}

type Enterprise struct {
	Enabled bool `yaml:"enabled"`
	// Commit pins Enterprise addons to an exact commit SHA, bypassing
	// branch/date resolution entirely. Takes priority over Date.
	Commit string `yaml:"commit"`
	// Date overrides which day to resolve the newest Enterprise commit
	// against (YYYYMMDD, same format as base.release). Defaults to
	// base.release when unset.
	Date string `yaml:"date"`
}

type Addons struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
	// SkipManifestValidation disables parsing each addon's __manifest__.py/
	// __openerp__.py content. The manifest file must still exist (that's
	// what identifies a directory as an addon); only its Python-literal
	// content goes unchecked. An escape hatch for manifests using syntax
	// ParseManifest doesn't yet support.
	SkipManifestValidation bool `yaml:"skip_manifest_validation"`
}

type Submodules struct {
	Init      bool `yaml:"init"`
	Recursive bool `yaml:"recursive"`
}

type Build struct {
	Platform []string `yaml:"platform"`
}

type Cache struct {
	Enabled bool `yaml:"enabled"`
}

type Image struct {
	Name string `yaml:"name"`
	Tag  string `yaml:"tag"`
}

// Default returns the configuration used when no odoo-builder.yaml is present
// (or as the base that a present odoo-builder.yaml is merged over).
func Default() *Config {
	return &Config{
		Addons: Addons{
			Include: []string{"addons"},
		},
	}
}
