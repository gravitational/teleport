/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsBrowserUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ua   string
		want bool
	}{
		{
			name: "empty",
			ua:   "",
			want: false,
		},
		{
			name: "tsh",
			ua:   "tsh/17.0.0",
			want: false,
		},
		{
			name: "curl",
			ua:   "curl/8.4.0",
			want: false,
		},
		{
			name: "Go http client",
			ua:   "Go-http-client/1.1",
			want: false,
		},
		{
			name: "Mozilla prefix without engine token",
			ua:   "Mozilla/5.0 compatible",
			want: false,
		},
		{
			name: "Chrome on macOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			want: true,
		},
		{
			name: "Safari on macOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.1",
			want: true,
		},
		{
			name: "Safari on iPhone",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 8_4 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) Version/8.0 Mobile/12H143 Safari/600.1.4",
			want: true,
		},
		{
			name: "Firefox on Linux",
			ua:   "Mozilla/5.0 (Linux; U; Linux 2.6; en-US; rv:1.9.9.2) Gecko/20100722 Firefox/3.6.8",
			want: true,
		},
		{
			name: "Firefox on iOS",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 12_0_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/7.0.4 Mobile/16A404 Safari/605.1.15",
			want: true,
		},
		{
			name: "Chrome on iOS",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 8_2 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) CriOS/44.0.2403.67 Mobile/12D508 Safari/600.1.4",
			want: true,
		},
		{
			name: "Edge on Windows",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.3912.72",
			want: true,
		},
		{
			name: "Opera on macOS",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 OPR/108.0.0.0",
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isBrowserUserAgent(tc.ua))
		})
	}
}

func TestWriteTrustedDeviceRequired(t *testing.T) {
	t.Parallel()

	const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

	tests := []struct {
		name            string
		userAgent       string
		wantContentType string
		wantBodyParts   []string
		wantNoBodyPart  string
	}{
		{
			name:            "browser gets HTML with clickable links to Web UI and app access guides",
			userAgent:       browserUA,
			wantContentType: "text/html; charset=utf-8",
			wantBodyParts: []string{
				`<a href="` + trustedDeviceRequiredWebUIDocsURL + `"`,
				`<a href="` + trustedDeviceRequiredAppAccessDocsURL + `"`,
			},
		},
		{
			name:            "non-browser gets plain text with URL but no HTML link",
			userAgent:       "tsh/17.0.0",
			wantContentType: "text/plain; charset=utf-8",
			wantBodyParts:   []string{trustedDeviceRequiredDocsURL},
			wantNoBodyPart:  "<a ",
		},
		{
			name:            "empty UA gets plain text with URL but no HTML link",
			userAgent:       "",
			wantContentType: "text/plain; charset=utf-8",
			wantBodyParts:   []string{trustedDeviceRequiredDocsURL},
			wantNoBodyPart:  "<a ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.userAgent != "" {
				req.Header.Set("User-Agent", tc.userAgent)
			}
			rec := httptest.NewRecorder()

			writeTrustedDeviceRequired(rec, req, http.StatusForbidden)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Equal(t, tc.wantContentType, rec.Header().Get("Content-Type"))
			for _, want := range tc.wantBodyParts {
				require.Contains(t, rec.Body.String(), want)
			}
			if tc.wantNoBodyPart != "" {
				require.NotContains(t, rec.Body.String(), tc.wantNoBodyPart)
			}
		})
	}
}
