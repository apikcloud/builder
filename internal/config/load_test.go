package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/config"
)

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "odoo-builder.yaml"))
	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestLoad_FullExample_Parses(t *testing.T) {
	path := writeFile(t, `
base:
  version: "18.0"
  release: "20250611"

enterprise:
  enabled: true

addons:
  include:
    - addons
    - addons-extra
  exclude:
    - test_module
  skip_manifest_validation: true

submodules:
  init: true
  recursive: true

build:
  platform:
    - linux/amd64
    - linux/arm64

cache:
  enabled: true

image:
  name: registry.example.com/customer/odoo
  tag: production

labels:
  org.opencontainers.image.vendor: Example

environment:
  MY_ENV: value
`)

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "18.0", cfg.Base.Version)
	assert.Equal(t, "20250611", cfg.Base.Release)
	assert.True(t, cfg.Enterprise.Enabled)
	assert.Equal(t, []string{"addons", "addons-extra"}, cfg.Addons.Include)
	assert.Equal(t, []string{"test_module"}, cfg.Addons.Exclude)
	assert.True(t, cfg.Addons.SkipManifestValidation)
	assert.True(t, cfg.Submodules.Init)
	assert.True(t, cfg.Submodules.Recursive)
	assert.Equal(t, []string{"linux/amd64", "linux/arm64"}, cfg.Build.Platform)
	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, "registry.example.com/customer/odoo", cfg.Image.Name)
	assert.Equal(t, "production", cfg.Image.Tag)
	assert.Equal(t, "Example", cfg.Labels["org.opencontainers.image.vendor"])
	assert.Equal(t, "value", cfg.Environment["MY_ENV"])
}

func TestLoad_MalformedYAML_ReturnsError(t *testing.T) {
	path := writeFile(t, "addons:\n  include: [unterminated\n")

	_, err := config.Load(path)
	require.Error(t, err)
}

func TestLoad_PartialYAML_MergesOverDefaults(t *testing.T) {
	path := writeFile(t, `
enterprise:
  enabled: true
`)

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.True(t, cfg.Enterprise.Enabled)
	assert.Equal(t, []string{"addons"}, cfg.Addons.Include, "unset fields must keep Default() values")
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "odoo-builder.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
