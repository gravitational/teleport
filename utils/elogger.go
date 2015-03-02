package utils

import (
	"github.com/codahale/lunk"
)

var NullEventLogger lunk.EventLogger = &NOPEventLogger{}

type NOPEventLogger struct {
}

func (*NOPEventLogger) Log(lunk.EventID, lunk.Event) {
}
