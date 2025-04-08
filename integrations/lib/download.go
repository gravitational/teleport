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

package lib

import (
	"context"
	"io"
	"net/http"

	"github.com/gravitational/trace"
)

// DownloadAndCheck gets a file from the Internet and checks its SHA256 sum.
func DownloadAndCheck(ctx context.Context, url string, out io.Writer, checksum SHA256Sum) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	sha256 := NewSHA256()
	if _, err = io.Copy(out, io.TeeReader(resp.Body, sha256)); err != nil {
		return trace.Wrap(err)
	}

	if sha256.Sum() != checksum {
		return trace.CompareFailed("sha256 sum of downloaded file does not match")
	}
	return nil
}
