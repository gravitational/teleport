// Package testenv provides common environment helpers for Antithesis workload
// tests.
package testenv

import (
	"cmp"
	"os"
	"path/filepath"
)

const (
	// DefaultCredsDir is the default directory where tbot writes workload identities.
	DefaultCredsDir = "/creds"

	// DefaultProxyAddr is the default Teleport proxy address for Antithesis workloads.
	DefaultProxyAddr = "antithesis.teleport.local:3080"

	// DefaultIdentityFile is the default identity filename under an identity directory.
	DefaultIdentityFile = "identity"
)

// CredsDir returns the directory where workload identities are stored.
func CredsDir() string {
	return cmp.Or(os.Getenv("TELEPORT_CREDS_DIR"), DefaultCredsDir)
}

// ProxyAddr returns the Teleport proxy address for workload clients.
func ProxyAddr() string {
	return cmp.Or(os.Getenv("TELEPORT_PROXY_ADDR"), DefaultProxyAddr)
}

// IdentityPath returns the identity file path for the named workload identity.
func IdentityPath(identity string) string {
	if path := os.Getenv("TELEPORT_IDENTITY_FILE"); path != "" {
		return path
	}
	return filepath.Join(CredsDir(), identity, DefaultIdentityFile)
}
