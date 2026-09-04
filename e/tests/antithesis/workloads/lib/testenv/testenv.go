// Package testenv provides common environment helpers for Antithesis workload
// tests.
package testenv

import (
	"cmp"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultCredsDir is the default directory where tbot writes workload identities.
	DefaultCredsDir = "/creds"

	// DefaultProxyAddr is the default Teleport proxy address for Antithesis workloads.
	DefaultProxyAddr = "antithesis.teleport.local:3080"

	// DefaultClusterName is the default Teleport cluster name for Antithesis workloads.
	DefaultClusterName = "antithesis.teleport.local"

	// DefaultIdentityFile is the default identity filename under an identity directory.
	DefaultIdentityFile = "identity"

	AuditEventEmitDeadline  = 20 * time.Minute
	MaxTolerableClockJitter = 10 * time.Minute
	// AuditEventSessionChunkTTL is the default Teleport TTL for emitting session events keyed on session ID.
	// When executing in parallel it is possible that another concurrent run of this test already emitted this event.
	AuditEventSessionChunkTTL = 5 * time.Minute
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
