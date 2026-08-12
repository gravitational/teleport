package directory

import (
	"strconv"

	"github.com/gravitational/teleport/e/lib/mdmsync"
)

// entraUniqueID is the Entra ID resource object ID.
type entraUniqueID string

// accessListName is the resource name of the Access List.
type accessListName string

// String returns the string representation of accessListName.
func (n accessListName) String() string {
	return string(n)
}

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
