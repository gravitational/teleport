/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"io"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/andybalholm/brotli"
)

var compressedFileExtensions = []string{
	".js",
	".svg",
	".wasm",
}

// makeBrotliHandler serves pre-compressed .br files for supported file types.
// Clients that do not accept brotli receive the decompressed contents when
// only the pre-compressed file exists.
func makeBrotliHandler(handler http.Handler, fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		isRequestForCompressedFile := slices.Contains(compressedFileExtensions, ext)
		if !isRequestForCompressedFile {
			handler.ServeHTTP(w, r)
			return
		}

		clientAcceptsBrotli := strings.Contains(r.Header.Get("Accept-Encoding"), "br")
		if !clientAcceptsBrotli {
			// Serve the original file if it exists (for example, files below
			// the compression threshold or builds without compression).
			if f, err := fs.Open(r.URL.Path); err == nil {
				f.Close()
				handler.ServeHTTP(w, r)
				return
			}
		}

		brPath := r.URL.Path + ".br"
		brFile, err := fs.Open(brPath)
		if err != nil {
			handler.ServeHTTP(w, r)
			return
		}
		defer brFile.Close()

		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream" // same default as http.DetectContentType
		}

		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Type", contentType)
		if clientAcceptsBrotli {
			w.Header().Set("Content-Encoding", "br")
		}

		if r.Method == http.MethodHead {
			return
		}

		if clientAcceptsBrotli {
			io.Copy(w, brFile)
			return
		}
		io.Copy(w, brotli.NewReader(brFile))
	})
}
