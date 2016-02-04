// +build appengine

package kingpin

import "io"

func guessWidth(w io.Writer) int {
	// No need to guess for appengine...
	return 80
}
