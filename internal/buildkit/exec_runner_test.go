// SPDX-License-Identifier: MIT
package buildkit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecRunner_Build_RejectsUnknownOutputType(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oci")
	assert.Contains(t, err.Error(), "docker")
	assert.Contains(t, err.Error(), "registry")
}

func TestExecRunner_Build_RegistryRequiresImage(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "registry", Image: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Image")
}

func TestExecRunner_Build_DockerRequiresImage(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "docker", Image: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Image")
}

func TestExecRunner_Build_BuildctlStdoutRoutedToStderr(t *testing.T) {
	dir := t.TempDir()
	buildctl := filepath.Join(dir, "buildctl")
	require.NoError(t, os.WriteFile(buildctl, []byte("#!/bin/sh\necho on-stdout\n"), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	_, buildErr := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "oci", OutputPath: filepath.Join(dir, "out.tar")})

	require.NoError(t, w.Close())
	captured, readErr := io.ReadAll(r)
	require.NoError(t, readErr)

	require.NoError(t, buildErr)
	assert.NotContains(t, string(captured), "on-stdout")
}

func TestExecRunner_Build_RegistryInsecureAppendsOutputOpt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		insecure bool
		wantOpt  bool
	}{
		{name: "insecure true appends opt", insecure: true, wantOpt: true},
		{name: "insecure false omits opt", insecure: false, wantOpt: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			capturedArgsPath := filepath.Join(dir, "captured-args.txt")
			buildctl := filepath.Join(dir, "buildctl")
			script := "#!/bin/sh\necho \"$@\" > " + capturedArgsPath + "\n"
			require.NoError(t, os.WriteFile(buildctl, []byte(script), 0o755))

			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

			_, err := execRunner{}.Build(context.Background(), BuildOptions{
				OutputType: "registry",
				Image:      "example.com/app:latest",
				Insecure:   tc.insecure,
			})
			require.NoError(t, err)

			captured, readErr := os.ReadFile(capturedArgsPath)
			require.NoError(t, readErr)

			if tc.wantOpt {
				assert.Contains(t, string(captured), "registry.insecure=true")
			} else {
				assert.NotContains(t, string(captured), "registry.insecure=true")
			}
		})
	}
}

func TestExecRunner_Build_TLSEnvAppendsFlags(t *testing.T) {
	dir := t.TempDir()
	capturedArgsPath := filepath.Join(dir, "captured-args.txt")
	buildctl := filepath.Join(dir, "buildctl")
	script := "#!/bin/sh\necho \"$@\" > " + capturedArgsPath + "\n"
	require.NoError(t, os.WriteFile(buildctl, []byte(script), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILDKIT_HOST", "tcp://buildkitd.example:1234")
	t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
	t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")
	t.Setenv("BUILDKIT_TLS_CACERT", "/tls/ca.pem")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{
		OutputType: "oci",
		OutputPath: filepath.Join(dir, "out.tar"),
	})
	require.NoError(t, err)

	captured, readErr := os.ReadFile(capturedArgsPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(captured), "--tlscert /tls/cert.pem --tlskey /tls/key.pem --tlscacert /tls/ca.pem")
}

func TestExecRunner_Build_TLSEnvUnsetOmitsFlags(t *testing.T) {
	dir := t.TempDir()
	capturedArgsPath := filepath.Join(dir, "captured-args.txt")
	buildctl := filepath.Join(dir, "buildctl")
	script := "#!/bin/sh\necho \"$@\" > " + capturedArgsPath + "\n"
	require.NoError(t, os.WriteFile(buildctl, []byte(script), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILDKIT_HOST", "unix:///tmp/nonexistent.sock")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{
		OutputType: "oci",
		OutputPath: filepath.Join(dir, "out.tar"),
	})
	require.NoError(t, err)

	captured, readErr := os.ReadFile(capturedArgsPath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(captured), "--tlscert")
}

func TestExecRunner_Build_TLSEnvPartialSetErrors(t *testing.T) {
	t.Setenv("BUILDKIT_HOST", "tcp://buildkitd.example:1234")
	t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "oci", OutputPath: t.TempDir() + "/out.tar"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must all be set together")
}

func TestExecRunner_Build_TLSEnvWithoutExternalHostErrors(t *testing.T) {
	t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
	t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")
	t.Setenv("BUILDKIT_TLS_CACERT", "/tls/ca.pem")

	_, err := execRunner{}.Build(context.Background(), BuildOptions{OutputType: "oci", OutputPath: t.TempDir() + "/out.tar"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-spawned buildkitd")
}
