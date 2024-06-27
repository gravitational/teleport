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
	"maps"
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

	require.Equal(t, "", GetAnyHeader(header))
	require.Equal(t, "", GetAnyHeader(header, "ccc"))
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

func TestSanitizeHeaders(t *testing.T) {
	innoffensiveHeaders := http.Header{
		"User-Agent":     {"Some Client v1.0.0"},
		"Accept":         {"text/json", "text/xml"},
		"Accept-Charset": {"utf-8"},
		"Connection":     {"Keep-Alive"},
	}

	inoffensiveHeadersPlus := func(extras ...string) http.Header {
		if len(extras) < 2 {
			panic("Must have at least 1 key/value pair")
		}

		if len(extras)%2 != 0 {
			panic("Must have at integral number of key-pair values")
		}

		dst := maps.Clone(innoffensiveHeaders)
		for i := 0; i < len(extras); i += 2 {
			dst[extras[i]] = []string{extras[i+1]}
		}
		return dst
	}

	equalsInoffensiveHeaders := func(t require.TestingT, value any, args ...any) {
		require.Equal(t, innoffensiveHeaders, value, args)
	}

	testCases := []struct {
		name      string
		input     http.Header
		assertion require.ValueAssertionFunc
	}{
		{
			name:      "Authorization is redacted",
			input:     inoffensiveHeadersPlus("Authorization", "Bearer SOME-TOKEN"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "Proxy-Authorization is redacted",
			input:     inoffensiveHeadersPlus("Proxy-Authorization", "Bearer SOME-TOKEN"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "Set-Cookie is redacted",
			input:     inoffensiveHeadersPlus("Set-Cookie", "blah"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "leading API-key is redacted",
			input:     inoffensiveHeadersPlus("api-key-here", "blah"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "trailing API-key is redacted",
			input:     inoffensiveHeadersPlus("some-api-key", "blah"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "infix API-key is redacted",
			input:     inoffensiveHeadersPlus("some-api-key-goes-here", "blah"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "X-Amz-Security-Token is redacted",
			input:     inoffensiveHeadersPlus("X-Amz-Security-Token", `70|<3|\|`),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name: "multiple matches are redacted",
			input: inoffensiveHeadersPlus(
				"Authorization", "Bearer SOME-TOKEN",
				"Proxy-Authorization", "Bearer SOME-TOKEN",
				"X-Amz-Security-Token", `70|<3|\|`),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "matching is case insensitive",
			input:     inoffensiveHeadersPlus("sET-cOOKIE", "blah blah blah"),
			assertion: equalsInoffensiveHeaders,
		},
		{
			name:      "handles empty headers",
			input:     http.Header{},
			assertion: require.Empty,
		},
		{
			name:      "handles nil",
			input:     nil,
			assertion: require.Empty,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			uut := SanitizeHeaders(tt.input)
			tt.assertion(t, uut)
		})
	}
}
