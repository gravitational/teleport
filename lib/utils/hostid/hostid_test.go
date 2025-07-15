//go:build !windows

// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package hostid_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestReadOrCreate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var wg errgroup.Group
	concurrency := 10
	ids := make([]string, concurrency)
	barrier := make(chan struct{})

	for i := range concurrency {
		wg.Go(func() error {
			<-barrier
			id, err := hostid.ReadOrCreateFile(
				dir,
				hostid.WithBackoff(retryutils.RetryV2Config{
					First:  50 * time.Millisecond,
					Driver: retryutils.NewExponentialDriver(100 * time.Millisecond),
					Max:    15 * time.Second,
					Jitter: retryutils.FullJitter,
				}),
				hostid.WithIterationLimit(10),
			)
			ids[i] = id
			return err
		})
	}

	close(barrier)

	require.NoError(t, wg.Wait())
	require.Equal(t, slices.Repeat([]string{ids[0]}, concurrency), ids)
}

func TestIdempotence(t *testing.T) {
	t.Parallel()

	// call twice, get same result
	dir := t.TempDir()
	id, err := hostid.ReadOrCreateFile(dir)
	require.Len(t, id, 36)
	require.NoError(t, err)
	uuidCopy, err := hostid.ReadOrCreateFile(dir)
	require.NoError(t, err)
	require.Equal(t, id, uuidCopy)
}

func TestBadLocation(t *testing.T) {
	t.Parallel()

	// call with a read-only dir, make sure to get an error
	id, err := hostid.ReadOrCreateFile("/bad-location")
	require.Empty(t, id)
	require.Error(t, err)
	require.Regexp(t, "^.*no such file or directory.*$", err.Error())
}

func TestIgnoreWhitespace(t *testing.T) {
	t.Parallel()

	// newlines are getting ignored
	dir := t.TempDir()
	id := fmt.Sprintf("%s\n", uuid.NewString())
	err := os.WriteFile(filepath.Join(dir, hostid.FileName), []byte(id), 0666)
	require.NoError(t, err)
	out, err := hostid.ReadFile(dir)
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(id), out)
}

func TestRegenerateEmpty(t *testing.T) {
	t.Parallel()

	// empty UUID in file is regenerated
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, hostid.FileName), nil, 0666)
	require.NoError(t, err)
	out, err := hostid.ReadOrCreateFile(dir)
	require.NoError(t, err)
	require.Len(t, out, 36)
}
