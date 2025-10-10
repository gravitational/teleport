package ttyterminal

import (
	"time"
)

type command struct {
	startTime, endTime time.Duration
	input, output      commandData
}

type commandData struct {
	startTime, endTime time.Duration
	startSize          size
	tokens             []token
	isAlternateScreen  bool
}
