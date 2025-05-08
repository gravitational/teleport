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

package utils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameHeaders(t *testing.T) {
	header := make(http.Header)
	header.Add("aaa", "a1")
	header.Add("aaa", "a2")
	header.Add("bbb", "b1")
	header.Add("ccc", "c1")

	RenameHeader(header, "aaa", "aaaa")
	RenameHeader(header, "bbb", "bbbb")
	RenameHeader(header, "ccc", "ccc")
	require.Equal(t, http.Header{
		"Aaaa": []string{"a1", "a2"},
		"Bbbb": []string{"b1"},
		"Ccc":  []string{"c1"},
	}, header)
}

func TestGetAnyHeader(t *testing.T) {
	header := make(http.Header)
	header.Set("aaa", "a1")
	header.Set("bbb", "b1")

	require.Empty(t, GetAnyHeader(header))
	require.Empty(t, GetAnyHeader(header, "ccc"))
	require.Equal(t, "a1", GetAnyHeader(header, "aaa"))
	require.Equal(t, "a1", GetAnyHeader(header, "ccc", "aaa"))
	require.Equal(t, "b1", GetAnyHeader(header, "bbb", "aaa"))
}

func TestGetSingleHeader(t *testing.T) {
	t.Run("NoValue", func(t *testing.T) {
		t.Parallel()
		headers := make(http.Header)

		result, err := GetSingleHeader(headers, "key")
		require.Empty(t, result)
		require.Error(t, err)
	})
	t.Run("SingleValue", func(t *testing.T) {
		t.Parallel()
		headers := make(http.Header)
		key := "key"
		value := "value"
		headers.Set(key, value)

		result, err := GetSingleHeader(headers, key)
		require.NoError(t, err)
		require.Equal(t, value, result)
	})
	t.Run("DuplicateValue", func(t *testing.T) {
		t.Parallel()
		headers := make(http.Header)
		key := "key"
		value := "value1"
		headers.Add(key, value)
		headers.Add(key, "value2")

		result, err := GetSingleHeader(headers, key)
		require.Empty(t, result)
		require.Error(t, err)
	})
	t.Run("DuplicateCaseValue", func(t *testing.T) {
		t.Parallel()
		headers := make(http.Header)
		key := "key"
		value := "value1"
		headers.Add(key, value)
		headers.Add(strings.ToUpper(key), "value2")

		result, err := GetSingleHeader(headers, key)
		require.Empty(t, result)
		require.Error(t, err)
	})
}

func TestChainHTTPMiddlewares(t *testing.T) {
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("baseHandler"))
	})

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("middleware2->"))
			next.ServeHTTP(w, r)
		})
	}
	middleware4 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("middleware4->"))
			next.ServeHTTP(w, r)
		})
	}

	handler := ChainHTTPMiddlewares(
		baseHandler,
		nil,
		middleware2,
		NoopHTTPMiddleware,
		middleware4,
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("", "/", nil)
	handler.ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "middleware4->middleware2->baseHandler", string(body))
}
