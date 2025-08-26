package identitycenter

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

const (
	// principalDeleteLabel is attached to a Principal State record to indicate
	// what should happen to the corresponding remote resources when the principal
	// is deleted. The absence of this label implies that the remote resource
	// should be deprovisioned as per normal.
	principalDeleteLabel = types.TeleportInternalLabelPrefix + "awsic/delete"

	// principalDeleteModeTeleportOnly indicates that the downstream resource
	// should be left as-is, while the Teleport resource should be deleted as
	// per normal.
	principalDeleteModeTeleportOnly = "teleport-only"

	// defaultResourceSyncInterval is the interval between synchronization passes that
	// pull data into Teleport from AWS
	defaultResourceSyncInterval = 5 * time.Minute

	// defaultEventEventBufferSize indicates how many events to buffer while
	// handling resources
	defaultEventEventBufferSize = 128
)

var (
	// DefaultFullAssignmentSyncInterval is the default interval between full
	// account assignment re-calculation and re-provisioning
	DefaultFullAssignmentSyncInterval = 10 * time.Minute
)
