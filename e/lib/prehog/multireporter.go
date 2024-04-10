package prehog

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// AddReporter adds a reporter to the given auth server, such that events passed
// to AnonymizeAndSubmit are fed to all the existing reporters and to the new
// one. Likewise, the graceful stop procedure will be ran on all the existing
// reporters and the new one - if supported.
func AddReporter(auth *auth.Server, reporter usagereporter.UsageReporter) {
	if auth.UsageReporter == nil {
		auth.SetUsageReporter(reporter)
		return
	}

	if _, isDiscard := auth.UsageReporter.(usagereporter.DiscardUsageReporter); isDiscard {
		auth.SetUsageReporter(reporter)
		return
	}

	if mr, isMultiReporter := auth.UsageReporter.(*multiReporter); isMultiReporter {
		mr.reporters = append(mr.reporters, reporter)
		return
	}

	auth.SetUsageReporter(&multiReporter{
		reporters: []usagereporter.UsageReporter{
			auth.UsageReporter,
			reporter,
		},
	})
}

// multiReporter is a [usagereporter.UsageReporter] that feeds all received
// events into a slice of reporters, in order.
type multiReporter struct {
	reporters []usagereporter.UsageReporter
}

var (
	_ usagereporter.UsageReporter   = (*multiReporter)(nil)
	_ usagereporter.GracefulStopper = (*multiReporter)(nil)
)

// AnonymizeAndSubmit implements [usagereporter.UsageReporter].
func (mr *multiReporter) AnonymizeAndSubmit(event ...usagereporter.Anonymizable) {
	for _, reporter := range mr.reporters {
		reporter.AnonymizeAndSubmit(event...)
	}
}

// GracefulStop implements [usagereporter.GracefulStopper].
func (mr *multiReporter) GracefulStop(ctx context.Context) error {
	errs := make([]error, 0, len(mr.reporters))
	for _, reporter := range mr.reporters {
		if gs, ok := reporter.(usagereporter.GracefulStopper); ok {
			errs = append(errs, gs.GracefulStop(ctx))
		}
	}
	return trace.NewAggregate(errs...)
}
