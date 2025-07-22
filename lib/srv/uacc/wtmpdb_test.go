package uacc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestingWtmpdb(t *testing.T) *wtmpdbBackend {
	dbFile := filepath.Join(t.TempDir(), "wtmp.db")
	wtmpdb, err := newWtmpdb(dbFile)
	require.NoError(t, err)
	return wtmpdb
}

func TestWtmpdb(t *testing.T) {
	t.Parallel()

}
