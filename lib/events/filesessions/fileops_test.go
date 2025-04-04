package filesessions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestPlainFileOpsReservations(t *testing.T) {
	ctx := context.Background()
	fops := NewPlainFileOps(utils.NewSlogLoggerForTests(), os.OpenFile)
	base := t.TempDir()
	reservation := filepath.Join(base, "reservation")
	var fileSize int64 = 512

	err := fops.CreateReservation(ctx, reservation, fileSize)
	require.NoError(t, err)

	info, err := os.Stat(reservation)
	require.NoError(t, err)
	require.Equal(t, fileSize, info.Size())

	buf := bytes.NewBufferString("testing")
	expectedLen := buf.Len()
	err = fops.WriteReservation(ctx, reservation, buf)
	require.NoError(t, err)

	info, err = os.Stat(reservation)
	require.NoError(t, err)
	require.Equal(t, int64(expectedLen), info.Size())
}

func TestPlainFileOpsCombineParts(t *testing.T) {
	ctx := context.Background()
	fops := NewPlainFileOps(utils.NewSlogLoggerForTests(), os.OpenFile)
	base := t.TempDir()
	parts := []string{"part1", "part2", "part3"}
	partPaths := make([]string, len(parts))
	for idx, part := range parts {
		partPaths[idx] = filepath.Join(base, part)
		f, err := os.Create(partPaths[idx])
		require.NoError(t, err)

		_, err = f.WriteString(part)
		require.NoError(t, err)
	}

	dst := bytes.NewBuffer(nil)
	err := fops.CombineParts(ctx, dst, partPaths)
	require.NoError(t, err)

	output, _ := dst.ReadString(0)

	require.Equal(t, strings.Join(parts, ""), output)
}
