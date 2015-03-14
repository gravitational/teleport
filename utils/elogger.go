package utils

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
)

var NullEventLogger lunk.EventLogger = &NOPEventLogger{}

type NOPEventLogger struct {
}

func (*NOPEventLogger) Log(lunk.EventID, lunk.Event) {
}
