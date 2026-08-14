package mdmsync

// SyncMode is a sync mode of operations for syncs.
type SyncMode int

const (
	// SyncModePartial represents a partial sync (e.g. entire MDM inventory not sent to
	// Teleport).
	SyncModePartial SyncMode = iota
	// SyncModeFull represents a full sync (e.g. entire MDM inventory sent to
	// Teleport).
	SyncModeFull
)
