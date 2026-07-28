// SPDX-License-Identifier: MIT
package buildkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheArgs(t *testing.T) {
	assert.Nil(t, cacheArgs("", ""))

	assert.Equal(t, []string{
		"--export-cache", "type=local,dest=/tmp/x,mode=max",
		"--import-cache", "type=local,src=/tmp/x",
	}, cacheArgs("", "/tmp/x"))

	got := cacheArgs("reg/app:buildcache", "/tmp/x")
	assert.Equal(t, []string{
		"--export-cache", "type=registry,ref=reg/app:buildcache,mode=max",
		"--import-cache", "type=registry,ref=reg/app:buildcache",
	}, got)
	for _, a := range got {
		assert.NotContains(t, a, "/tmp/x")
	}
}

func TestPlatformArgs(t *testing.T) {
	assert.Nil(t, platformArgs(nil))
	assert.Equal(t, []string{"--opt", "platform=linux/amd64"}, platformArgs([]string{"linux/amd64"}))
	assert.Equal(t, []string{"--opt", "platform=linux/amd64,linux/arm64"}, platformArgs([]string{"linux/amd64", "linux/arm64"}))
}

func TestImageResolveModeArgs(t *testing.T) {
	assert.Equal(t, []string{"--opt", "image-resolve-mode=local"}, imageResolveModeArgs())
}
