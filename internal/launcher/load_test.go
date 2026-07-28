package launcher

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_ReturnsCommandError(t *testing.T) {
	err := Load(context.Background(), Runtime("/nonexistent-binary-xyz"), "/nonexistent.tar", io.Discard, io.Discard)
	require.Error(t, err)
}
