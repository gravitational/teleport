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
)

var compressedFileExtensions = []string{
	".js",
	".svg",
	".wasm",
}

// makeBrotliHandler serves pre-compressed .br files for supported file types.
func makeBrotliHandler(handler http.Handler, fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		isRequestForCompressedFile := slices.Contains(compressedFileExtensions, ext)
		clientAcceptsBrotli := strings.Contains(r.Header.Get("Accept-Encoding"), "br")
		if !isRequestForCompressedFile || !clientAcceptsBrotli {
			handler.ServeHTTP(w, r)
			return
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

		w.Header().Set("Content-Encoding", "br")
		w.Header().Set("Content-Type", contentType)

		if r.Method == http.MethodHead {
			return
		}

		io.Copy(w, brFile)
	})
}
