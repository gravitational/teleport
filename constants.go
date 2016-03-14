package teleport

import (
	"time"
)

// ForeverTTL means that object TTL will not expire unless deleted
const ForeverTTL time.Duration = 0

const (
	// BoltBackendType is a BoltDB backend
	BoltBackendType = "bolt"

	// ETCDBackendType is etcd backend
	ETCDBackendType = "etcd"
)

const (
	// DebugOutputEnvVar tells tests to use verbose debug output
	DebugOutputEnvVar = "TELEPORT_DEBUG_TESTS"
)
