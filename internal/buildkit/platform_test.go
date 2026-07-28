// SPDX-License-Identifier: MIT
package buildkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePlatform(t *testing.T) {
	for _, s := range []string{"linux/amd64", "linux/arm64", "linux/arm/v7"} {
		assert.NoError(t, ValidatePlatform(s), s)
	}

	for _, s := range []string{"amd64", "linux/", "linux /amd64", ""} {
		err := ValidatePlatform(s)
		if assert.Error(t, err, s) {
			assert.Contains(t, err.Error(), s)
		}
	}
}
