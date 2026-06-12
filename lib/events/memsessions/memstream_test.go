/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package memsessions

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	// Stream with handler and many parts
	t.Run("StreamManyParts", func(t *testing.T) {
		test.StreamManyParts(t, eventstest.NewMemoryUploader())
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, eventstest.NewMemoryUploader())
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, eventstest.NewMemoryUploader())
	})
}
