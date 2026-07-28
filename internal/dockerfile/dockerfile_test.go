// SPDX-License-Identifier: MIT
package dockerfile_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/dockerfile"
)

func TestGenerate_Minimal(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, false, false)

	assert.Equal(t, "FROM odoo:18.0-20260723\n\nCOPY addons/ /mnt/extra-addons/\n", out)
	assert.NotContains(t, out, "ENTRYPOINT")
	assert.NotContains(t, out, "CMD")
}

func TestGenerate_WithRequirements(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, true, false, false)

	assert.Contains(t, out, "COPY requirements.txt /tmp/requirements.txt\n")
	assert.Contains(t, out, "RUN PIP_BREAK_SYSTEM_PACKAGES=1 python3 -m pip install --no-cache-dir -r /tmp/requirements.txt\n")
	assert.Contains(t, out, "USER root\n")
	assert.Contains(t, out, "USER odoo\n")
	assert.NotContains(t, out, "packages.txt")

	userRootIdx := strings.Index(out, "USER root\n")
	runPipIdx := strings.Index(out, "RUN PIP_BREAK_SYSTEM_PACKAGES=1")
	userOdooIdx := strings.Index(out, "USER odoo\n")
	assert.Less(t, userRootIdx, runPipIdx, "USER root must precede the pip install RUN")
	assert.Less(t, runPipIdx, userOdooIdx, "USER odoo must follow the pip install RUN")
}

func TestGenerate_WithPackages(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, true, false)

	assert.Contains(t, out, "COPY packages.txt /tmp/packages.txt\n")
	assert.Contains(t, out, "USER root\n")
	assert.Contains(t, out, "RUN DEBIAN_FRONTEND=noninteractive apt-get update \\\n")
	assert.Contains(t, out, "    && grep -vE '^[[:space:]]*(#|$)' /tmp/packages.txt | xargs apt-get install -y --no-install-recommends \\\n")
	assert.Contains(t, out, "    && rm -rf /var/lib/apt/lists/*\n")
	assert.Contains(t, out, "USER odoo\n")
	assert.NotContains(t, out, "requirements.txt")

	userRootIdx := strings.Index(out, "USER root\n")
	runAptIdx := strings.Index(out, "RUN DEBIAN_FRONTEND=noninteractive apt-get update")
	userOdooIdx := strings.Index(out, "USER odoo\n")
	assert.Less(t, userRootIdx, runAptIdx, "USER root must precede the apt-get RUN")
	assert.Less(t, runAptIdx, userOdooIdx, "USER odoo must follow the apt-get RUN")
}

func TestGenerate_WithRequirementsAndPackages_SingleRootToggle(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, true, true, false)

	assert.Equal(t, 1, strings.Count(out, "USER root\n"), "expected a single USER root switch covering both blocks")
	assert.Equal(t, 1, strings.Count(out, "USER odoo\n"), "expected a single USER odoo switch back")
}

func TestGenerate_WithRequirementsAndPackages_PackagesInstallBeforePip(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, true, true, false)

	runAptIdx := strings.Index(out, "RUN DEBIAN_FRONTEND=noninteractive apt-get update")
	runPipIdx := strings.Index(out, "RUN PIP_BREAK_SYSTEM_PACKAGES=1")
	require.NotEqual(t, -1, runAptIdx)
	require.NotEqual(t, -1, runPipIdx)
	assert.Less(t, runAptIdx, runPipIdx, "packages.txt's apt-get install must run before requirements.txt's pip install, since pip requirements (e.g. VCS deps needing git) may depend on system packages")
}

func TestGenerate_NoRequirementsNoPackages(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, false, false)

	assert.NotContains(t, out, "requirements.txt")
	assert.NotContains(t, out, "packages.txt")
}

func TestGenerate_WithEnterpriseAddons_CopiedIntoOdooCoreAddonsDir(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, false, true)

	assert.Contains(t, out, "COPY addons/ /mnt/extra-addons/\n")
	assert.Contains(t, out, "COPY enterprise-addons/ /usr/lib/python3/dist-packages/odoo/addons/\n")
	assert.NotContains(t, out, "USER root\n", "the COPY needs no runtime USER switch: Docker COPY writes as root regardless of the current USER")
	assert.NotContains(t, out, "addons_path", "no odoo.conf change needed: this directory is already scanned by Odoo")

	communityIdx := strings.Index(out, "COPY addons/ /mnt/extra-addons/\n")
	enterpriseIdx := strings.Index(out, "COPY enterprise-addons/ /usr/lib/python3/dist-packages/odoo/addons/\n")
	assert.Less(t, communityIdx, enterpriseIdx, "community addons must be copied before enterprise addons")
}

func TestGenerate_NoEnterpriseAddons_NoEnterpriseCopy(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, false, false)

	assert.NotContains(t, out, "enterprise-addons")
	assert.NotContains(t, out, "dist-packages")
}

func TestGenerate_LabelsAndEnvironmentSortedByKey(t *testing.T) {
	cfg := &config.Config{
		Labels: map[string]string{
			"maintainer": "team",
			"build.id":   "42",
		},
		Environment: map[string]string{
			"PATH_EXTRA": "/opt/odoo",
			"ODOO_ENV":   "prod",
		},
	}

	out := dockerfile.Generate("odoo:18.0-20260723", cfg, false, false, false)

	labelIdx := strings.Index(out, `LABEL "build.id"="42"`)
	maintainerIdx := strings.Index(out, `LABEL "maintainer"="team"`)
	require := assert.New(t)
	require.NotEqual(-1, labelIdx)
	require.NotEqual(-1, maintainerIdx)
	require.Less(labelIdx, maintainerIdx)

	envOdooIdx := strings.Index(out, `ENV ODOO_ENV="prod"`)
	envPathIdx := strings.Index(out, `ENV PATH_EXTRA="/opt/odoo"`)
	require.NotEqual(-1, envOdooIdx)
	require.NotEqual(-1, envPathIdx)
	require.Less(envOdooIdx, envPathIdx)

	out2 := dockerfile.Generate("odoo:18.0-20260723", cfg, false, false, false)
	assert.Equal(t, out, out2)
}

func TestGenerate_EmptyLabelsAndEnvironment_NoStrayLines(t *testing.T) {
	out := dockerfile.Generate("odoo:18.0-20260723", &config.Config{}, false, false, false)

	assert.NotContains(t, out, "LABEL")
	assert.NotContains(t, out, "ENV ")
	assert.True(t, strings.HasSuffix(out, "/mnt/extra-addons/\n"))
}
