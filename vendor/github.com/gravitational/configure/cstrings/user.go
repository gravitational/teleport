package cstrings

import (
	"strings"
	"unicode"
)

const (
	maxUsernameLen = 32
)

// IsValidUnixUser returns true if passed string can be used
// to create a valid UNIX user
func IsValidUnixUser(u string) bool {
	/* See http://www.unix.com/man-page/linux/8/useradd:

		On Debian, the only constraints are that usernames must neither start with a dash ('-')
	    nor contain a colon (':') or a whitespace (space: ' ', end of line: '\n', tabulation:
	    '\t', etc.). Note that using a slash ('/') may break the default algorithm for the
	    definition of the user's home directory.

	*/
	if len(u) > maxUsernameLen || len(u) == 0 || u[0] == '-' {
		return false
	}
	if strings.ContainsAny(u, ":/") {
		return false
	}
	for _, r := range u {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
