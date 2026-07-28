// SPDX-License-Identifier: MIT
// Package registry resolves and validates the image references used when
// BuildKit pushes to an OCI-compatible registry (README.md's Milestone 5).
// It contains no authentication logic: buildctl resolves registry
// credentials ambiently from the host's docker config
// (~/.docker/config.json or $DOCKER_CONFIG) — populating that file is the
// entry point's responsibility (README's Local CLI section,
// README.md:528-556), not the engine's.
package registry

import (
	"fmt"
	"strings"
)

// Reference returns the full "name:tag" image reference for a odoo-builder.yaml
// image.name/image.tag pair. tag defaults to "latest" when empty.
func Reference(name, tag string) string {
	if tag == "" {
		tag = "latest"
	}
	return name + ":" + tag
}

// Validate reports whether name is usable as odoo-builder.yaml's image.name: a
// non-empty registry/repository path with no whitespace, and no embedded
// ":tag" or "@digest" — those belong in the separate image.tag field (or
// aren't supported at all). Accepting a name like
// "registry.example.com/app:v1" would silently make image.tag's value
// pointless, so it is rejected here instead of failing confusingly at
// push time.
func Validate(name string) error {
	if name == "" {
		return fmt.Errorf("registry: image name is empty")
	}
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("registry: image name %q contains whitespace", name)
	}
	if strings.Contains(name, "@") {
		return fmt.Errorf("registry: image name %q must not contain a digest (@...); configure image.tag separately", name)
	}
	// A colon after the last slash is a tag (e.g. "app:v1"); a colon at or
	// before the last slash is a registry host's port (e.g.
	// "localhost:5000/app"), which is valid and must be allowed.
	if i := strings.LastIndexByte(name, ':'); i > strings.LastIndexByte(name, '/') {
		return fmt.Errorf("registry: image name %q must not contain a tag (:...); configure image.tag separately", name)
	}
	return nil
}
