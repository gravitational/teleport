/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cstrings

import (
	"bufio"
	"strings"
	"unicode/utf8"
)

// SplitComma splits a comma-delimted string, comma can be escaped
// using '\' escape rune.
func SplitComma(v string) []string {
	return Split(',', '\\', v)
}

// Split splits the string v using delimiter rune. Rune can be escaped
// if prefixed with the escape rune.
func Split(delim, escape rune, v string) []string {
	if delim == 0 || escape == 0 {
		return []string{v}
	}
	scanner := bufio.NewScanner(strings.NewReader(v))
	s := &splitter{delim: delim, escape: escape}
	scanner.Split(s.scan)

	out := []string{}
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out
}

// SplitAt splits array of strings at a given separator
func SplitAt(args []string, sep string) ([]string, []string) {
	index := -1
	for i, a := range args {
		if a == sep {
			index = i
			break
		}
	}
	if index == -1 {
		return args, []string{}
	}
	return args[:index], args[index+1:]
}

type splitter struct {
	delim  rune
	escape rune
	prev   rune
}

func (s *splitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// scan until first unescaped delimiter and return token
	idx := 0
	var r rune
	for width := 0; idx < len(data); idx += width {
		r, width = utf8.DecodeRune(data[idx:])
		if r == s.delim && s.prev != s.escape && idx != 0 {
			s.prev = r
			return idx + width, data[:idx], nil
		}
		s.prev = r
	}

	// If we're at EOF, we have a final, non-empty, non-terminated chunk
	if atEOF && idx != 0 {
		return len(data), data[:idx], nil
	}
	// Request more data.
	return 0, nil, nil
}
