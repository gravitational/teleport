//go:build dynamodb
// +build dynamodb

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

package s3sessions

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
)

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	handler, err := NewHandler(context.Background(), Config{
		Region: "us-west-1",
		Path:   "/test/",
		Bucket: "teleport-unit-tests",
	})
	require.NoError(t, err)

	defer handler.Close()

	// Stream with handler and many parts
	t.Run("StreamSinglePart", func(t *testing.T) {
		test.StreamSinglePart(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}

func TestACL(t *testing.T) {
	t.Parallel()
	baseUrl := "s3://mybucket/path"
	for _, tc := range []struct {
		desc, acl string
		isError   bool
	}{
		{"no ACL", "", false},
		{"correct ACL", "bucket-owner-full-control", false},
		{"incorrect ACL", "something-else", true},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			url, err := url.Parse(fmt.Sprintf("%s?acl=%s", baseUrl, tc.acl))
			require.NoError(t, err)
			conf := Config{}
			err = conf.SetFromURL(url, "")
			if tc.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.acl, conf.ACL)
			}
		})
	}
}
