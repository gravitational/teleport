package identitycenter

import "time"

const (
	// defaultResourceSyncInterval is the interval between synchronization passes that
	// pull data into Teleport from AWS
	defaultResourceSyncInterval = 5 * time.Minute
)
