package backend

import (
	"time"
)

const (
	// DefaultBufferCapacity is a default circular buffer size
	// used by backends to fan out events
	DefaultBufferCapacity = 1024
	// DefaultBacklogGracePeriod is the default amount of time
	// that the circular buffer will tolerate an event backlog
	// in one of its watchers.
	DefaultBacklogGracePeriod = time.Second * 30
	// DefaultPollStreamPeriod is a default event poll stream period
	DefaultPollStreamPeriod = time.Second
	// DefaultEventsTTL is a default events TTL period
	DefaultEventsTTL = 10 * time.Minute
	// DefaultRangeLimit is used to specify some very large limit when limit is not specified
	// explicitly to prevent OOM due to infinite loops or other issues along those lines.
	DefaultRangeLimit = 200_000
)
