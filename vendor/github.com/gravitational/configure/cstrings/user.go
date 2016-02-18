package cstrings

import (
	"regexp"
)

var usersRe = regexp.MustCompile("^[a-z_][a-z0-9_-]*$")

// IsValidUnixUser returns true if passed string appears to
func IsValidUnixUser(u string) bool {
	if len(u) > 32 {
		return false
	}
	return usersRe.MatchString(u)
}
