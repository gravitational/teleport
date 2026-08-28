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

package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMakeCacheHandler(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	etag := "test-etag"

	recorder := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/testfile.woff", nil)
	if err != nil {
		t.Fatal(err)
	}

	cacheHandler := makeCacheHandler(testHandler, etag)

	cacheHandler.ServeHTTP(recorder, req)

	expectedCacheControl := "max-age=" + strconv.Itoa(int(time.Hour*24*365/time.Second)) + ", immutable"
	require.Equal(t, expectedCacheControl, recorder.Header().Get("Cache-Control"))

	req2, err := http.NewRequest("GET", "/testfile.css", nil)
	if err != nil {
		t.Fatal(err)
	}

	cacheHandler.ServeHTTP(recorder, req2)

	require.Equal(t, etag, recorder.Header().Get("ETag"))
}
