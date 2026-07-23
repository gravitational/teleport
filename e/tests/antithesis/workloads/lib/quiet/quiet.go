package quiet

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

const (
	stopFaultsEnv            = "ANTITHESIS_STOP_FAULTS"
	stopFaultsCommandTimeout = 30 * time.Second
)

// RequestQuietPeriod asks Antithesis to stop injected faults for duration.
// Within the simulator, Antithesis sets the env var ANTITHESIS_STOP_FAULTS to a command
// that can be executed to request quiet period (fault injection pause).
// When ran outside of Antithesis, the env var is not expected to be set and this function is a nop.
//
// See https://antithesis.com/docs/environment/fault_injection/pause_faults/
func RequestQuietPeriod(ctx context.Context, duration time.Duration) error {
	stopCommand := os.Getenv(stopFaultsEnv)
	if stopCommand == "" {
		// Nothing to do.
		return nil
	}

	if duration <= 0 {
		return trace.BadParameter("duration cannot be less or equal to 0")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, stopFaultsCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, stopCommand, strconv.FormatInt(int64(duration.Seconds()), 10))
	if _, err := cmd.CombinedOutput(); err != nil {
		return trace.Wrap(err, "requesting quiet period")
	}

	return nil
}
