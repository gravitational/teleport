// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import "strings"

// MarkdownEscape wraps some text `t` in triple backticks (escaping any backtick
// inside the message), limiting the length of the message to `n` runes (inside
// the single preformatted block). The text is trimmed before escaping.
// Backticks are escaped and thus count as two runes for the purpose of the
// truncation.
func MarkdownEscape(t string, n int) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "(empty)"
	}
	var b strings.Builder
	b.WriteString("```\n")
	for i, r := range t {
		if i >= n {
			b.WriteString("``` (truncated)")
			return b.String()
		}
		b.WriteRune(r)
		if r == '`' {
			// byte order mark, as a zero width no-break space; seems to result
			// in escaped backticks with no spurious characters in the message
			b.WriteRune('\ufeff')
			n--
		}
	}
	b.WriteString("```")
	return b.String()
}
