package identitycenter

import "time"

const (
	// defaultResourceSyncInterval is the interval between synchronization passes that
	// pull data into Teleport from AWS
	defaultResourceSyncInterval = 5 * time.Minute

	// defaultEventEventBufferSize indicates how many events to buffer while
	// handling resources
	defaultEventEventBufferSize = 128
)
