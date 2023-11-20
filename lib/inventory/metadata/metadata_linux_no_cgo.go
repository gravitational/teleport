//go:build linux && !cgo
// +build linux,!cgo

package metadata

import (
	log "github.com/sirupsen/logrus"
)

// fetchOSVersion returns "" if not on linux and not on darwin.
func (c *fetchConfig) fetchOSVersion() string {
	log.Warningf("fetchOSVersion is not implemented for builds without cgo")
	return ""
}

// fetchGlibcVersion returns "" if not on linux and not on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	log.Warningf("fetchGlibcVersion is not implemented for builds without cgo")
	return ""
}
