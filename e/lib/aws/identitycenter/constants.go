package identitycenter

import "time"

const (
	// defaultResourceSyncInterval is the interval between synchronization passes that
	// pull data into Teleport from AWS
	defaultResourceSyncInterval = 5 * time.Minute

	// defaultAssignmentSyncInterval is the default interval between full
	// account assignment re-calculation and re-provisioning
	defaultAssignmentSyncInterval = 10 * time.Minute

	// defaultEventEventBufferSize indicates how many events to buffer while
	// handling resources
	defaultEventEventBufferSize = 128
)
