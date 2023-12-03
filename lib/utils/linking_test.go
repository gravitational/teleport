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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebLinks(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		inResponse *http.Response
		outNext    string
		outPrev    string
		outFirst   string
		outLast    string
	}{
		// 0 - Multiple links in single header. Partial list of relations.
		{
			inResponse: &http.Response{
				Header: http.Header{
					"Link": []string{`<https://api.github.com/user/teams?page=2>; rel="next",
		                              <https://api.github.com/user/teams?page=34>; rel="last"`},
				},
			},
			outNext:  "https://api.github.com/user/teams?page=2",
			outPrev:  "",
			outFirst: "",
			outLast:  "https://api.github.com/user/teams?page=34",
		},
		// 1 - Multiple links in single header. Full list of relations.
		{
			inResponse: &http.Response{
				Header: http.Header{
					"Link": []string{`<https://api.github.com/user/teams?page=2>; rel="next",
		                              <https://api.github.com/user/teams?page=1>; rel="prev",
                                      <https://api.github.com/user/teams?page=1>; rel="first",
		                              <https://api.github.com/user/teams?page=34>; rel="last"`},
				},
			},
			outNext:  "https://api.github.com/user/teams?page=2",
			outPrev:  "https://api.github.com/user/teams?page=1",
			outFirst: "https://api.github.com/user/teams?page=1",
			outLast:  "https://api.github.com/user/teams?page=34",
		},
		// 2 - Multiple links in multiple headers. Full list of relations.
		{
			inResponse: &http.Response{
				Header: http.Header{
					"Link": []string{
						`<https://api.github.com/user/teams?page=1>; rel="next"`,
						`<https://api.github.com/user/teams?page=2>; rel="prev"`,
						`<https://api.github.com/user/teams?page=3>; rel="first"`,
						`<https://api.github.com/user/teams?page=4>; rel="last"`,
					},
				},
			},
			outNext:  "https://api.github.com/user/teams?page=1",
			outPrev:  "https://api.github.com/user/teams?page=2",
			outFirst: "https://api.github.com/user/teams?page=3",
			outLast:  "https://api.github.com/user/teams?page=4",
		},
	}

	for _, tt := range tests {
		wls := ParseWebLinks(tt.inResponse)
		require.Equal(t, wls.NextPage, tt.outNext)
		require.Equal(t, wls.PrevPage, tt.outPrev)
		require.Equal(t, wls.FirstPage, tt.outFirst)
		require.Equal(t, wls.LastPage, tt.outLast)
	}
}
