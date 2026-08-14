// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

//go:build !webassets_embed

// This file replaces webassets_noembed.go in the Teleport source tree during
// Antithesis harness builds. Instead of returning an error, it returns a minimal
// filesystem that serves a stub index.html so the
// proxy web server initializes and its API routes (e.g. /webapi/ping) work
// without embedded web UI assets.

package teleport

import (
	"net/http"
	"testing/fstest"
)

// NewWebAssetsFilesystem returns a minimal stub HTTP filesystem.
// The proxy service starts normally; only the static web UI assets are absent.
func NewWebAssetsFilesystem() (http.FileSystem, error) {
	return http.FS(fstest.MapFS{
		"index.html": {
			Data: []byte(`<!DOCTYPE html><html lang="en"><head>  <meta charset="UTF-8"><title>Teleport Antithesis Harness</title></head><body>  Teleport Antithesis harness, no web assets embedded.</body></html>`),
			Mode: 0444,
		},
		"apphash": {
			Data: []byte(`antithesis`),
			Mode: 0444,
		},
	}), nil
}
