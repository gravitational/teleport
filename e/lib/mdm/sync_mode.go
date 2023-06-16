package mdm

// SyncMode is a sync mode of operations for inventory syncs.
type SyncMode int

const (
	// SyncModePartial represents a partial sync (entire MDM inventory not sent to
	// Teleport).
	SyncModePartial SyncMode = iota
	// SyncModeFull represents a full sync (entire MDM inventory sent to
	// Teleport).
	SyncModeFull
)
