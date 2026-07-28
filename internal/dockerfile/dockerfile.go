// SPDX-License-Identifier: MIT
package dockerfile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apikcloud/odoo-builder/internal/config"
)

// enterpriseAddonsTarget is where Enterprise addons are copied inside the
// image: directly into Odoo's own core addons directory, alongside the
// community modules already shipped there, rather than a custom
// addons_path entry. This directory is already scanned by Odoo with no
// odoo.conf change needed, matching the reference odoo/odoo image's own
// Dockerfile layout.
const enterpriseAddonsTarget = "/usr/lib/python3/dist-packages/odoo/addons"

// Generate renders the full Dockerfile content for a build context whose
// base image has already been resolved to baseImage. hasRequirements/
// hasPackages/hasEnterpriseAddons indicate whether requirements.txt/
// packages.txt/an enterprise-addons/ directory were populated in the build
// directory by Prepare.
func Generate(baseImage string, cfg *config.Config, hasRequirements, hasPackages, hasEnterpriseAddons bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "FROM %s\n", baseImage)
	b.WriteString("\nCOPY addons/ /mnt/extra-addons/\n")
	if hasEnterpriseAddons {
		fmt.Fprintf(&b, "COPY enterprise-addons/ %s/\n", enterpriseAddonsTarget)
	}

	needsRoot := hasRequirements || hasPackages
	if needsRoot {
		b.WriteString("\nUSER root\n")
	}

	if hasPackages {
		b.WriteString("\nCOPY packages.txt /tmp/packages.txt\n")
		b.WriteString("RUN DEBIAN_FRONTEND=noninteractive apt-get update \\\n")
		// packages.txt may contain blank lines and "#"-prefixed comments
		// (internal/prepare/validate.go's validatePackagesTxt allows them);
		// grep strips both before xargs sees the file, otherwise a comment
		// line would be passed to apt-get install as a bogus package name.
		b.WriteString("    && grep -vE '^[[:space:]]*(#|$)' /tmp/packages.txt | xargs apt-get install -y --no-install-recommends \\\n")
		b.WriteString("    && rm -rf /var/lib/apt/lists/*\n")
	}

	if hasRequirements {
		b.WriteString("\nCOPY requirements.txt /tmp/requirements.txt\n")
		b.WriteString("RUN PIP_BREAK_SYSTEM_PACKAGES=1 python3 -m pip install --no-cache-dir -r /tmp/requirements.txt\n")
	}

	if needsRoot {
		b.WriteString("\nUSER odoo\n")
	}

	if len(cfg.Labels) > 0 {
		b.WriteString("\n")
		for _, key := range sortedKeys(cfg.Labels) {
			fmt.Fprintf(&b, "LABEL %q=%q\n", key, cfg.Labels[key])
		}
	}

	if len(cfg.Environment) > 0 {
		b.WriteString("\n")
		for _, key := range sortedKeys(cfg.Environment) {
			fmt.Fprintf(&b, "ENV %s=%q\n", key, cfg.Environment[key])
		}
	}

	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
