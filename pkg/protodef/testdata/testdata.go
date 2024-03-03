// Package testdata holds test data.
package testdata

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed *
var td embed.FS

// Bytes returns the content of a file from the testdata directory.
func Bytes(t *testing.T, fname string) []byte {
	t.Helper()
	b, err := td.ReadFile(fname)
	require.NoError(t, err)
	return b
}

// String returns the content of a file from the testdata directory.
func String(t *testing.T, fname string) string {
	t.Helper()
	return string(Bytes(t, fname))
}
