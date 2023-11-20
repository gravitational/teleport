//go:build !windows && !cgo
// +build !windows,!cgo

package shell

import "github.com/gravitational/trace"

func getLoginShell(username string) (string, error) {
	return "", trace.BadParameter("login shell requires cgo")
}
