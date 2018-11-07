package backend

import (
	"time"
)

const (
	// DefaultBufferSize is a default circular buffer size
	// used by dynamodb
	DefaultBufferSize = 1096
	// DefaultPollStreamPeriod is a default event poll stream period
	DefaultPollStreamPeriod = time.Second
	// DefaultEventsTTL is a default events TTL period
	DefaultEventsTTL = 10 * time.Minute
	// DefaultLargeLimit is used to specify some very large limit when limit is not specified
	// explicitly to prevent OOM
	DefaultLargeLimit = 30000
)
