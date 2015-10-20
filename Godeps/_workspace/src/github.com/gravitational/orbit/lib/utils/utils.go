package utils

import (
	"bufio"
	"strings"
	"unicode/utf8"
)

func SplitComma(v string) []string {
	return Split(',', '\\', v)
}

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
