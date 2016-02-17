package teleport

import (
	"time"
)

// ForeverTTL means that object TTL will not expire unless deleted
var ForeverTTL time.Duration = 0
