package directory

import (
	"strconv"

	"github.com/gravitational/teleport/e/lib/mdmsync"
)

// entraUniqueID is the Entra ID resource object ID.
type entraUniqueID string

// FriendlySyncMode converts [mdmsync.SyncMode] to `full` and `delta` string literals.
func FriendlySyncMode(mode mdmsync.SyncMode) string {
	switch mode {
	case mdmsync.SyncModeFull:
		return "full"
	case mdmsync.SyncModePartial:
		return "delta"
	default:
		// Could be a new unknown mode.
		return strconv.Itoa(int(mode))
	}
}
