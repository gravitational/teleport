package accesslist

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Hard coded hour reporting time. It's unlikely that an access list going out of compliance
// (i.e. going past the next audit date without a review) will happen particularly frequently,
// so this is likely sufficient.
const timeBetweenComplianceReports = 1 * time.Hour

// ReportCompliance will report compliance metrics to posthog periodically.
func (s *Service) ReportCompliance(ctx context.Context) {
	// Can't report anything if usage reporter is nil.
	if s.usageReporter == nil {
		return
	}

	s.log.Debug("Reporting Access List review compliance.")

	ticker := interval.New(interval.Config{
		Duration: timeBetweenComplianceReports,
		// We'll wait 30 seconds before reporting first in hopes that the server's usage reporter
		// has been set up properly. This isn't critical, so if this is missed it'll just be
		// picked up on the next tick.
		FirstDuration: time.Second * 30,
		Jitter:        retryutils.NewSeventhJitter(),
		Clock:         s.clock,
	})
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Next():
		case <-ctx.Done():
			return
		}
		if err := s.reportComplianceMetrics(ctx); err != nil {
			s.log.WithError(err).Error("Error reporting compliance metrics")
		}
	}
}

// reportComplianceMetrics will calculate and report the compliance metrics to posthog.
func (s *Service) reportComplianceMetrics(ctx context.Context) error {
	totalAccessLists, accessListsNeedReview, err := s.calculateComplianceMetrics(ctx)
	if err == nil {
		s.emitAccessListReviewComplianceUsageEvent(ctx, totalAccessLists, accessListsNeedReview)
	}

	return trace.Wrap(err)
}

// calculateComplianceMetrics will calculate metrics relevant to report compliance of access list reviews.
func (s *Service) calculateComplianceMetrics(ctx context.Context) (totalAccessLists, accessListsNeedReview int, err error) {
	now := s.clock.Now()

	var token string
	for {
		var page []*accesslist.AccessList
		page, token, err = s.accessLists.ListAccessLists(ctx, 0 /* page size */, token)
		if err != nil {
			return 0, 0, trace.Wrap(err)
		}

		totalAccessLists += len(page)
		for _, accessList := range page {
			if accessList.Spec.Audit.NextAuditDate.Before(now) {
				accessListsNeedReview++
			}
		}

		if token == "" {
			break
		}
	}

	return totalAccessLists, accessListsNeedReview, nil
}

// emitAccessListReviewComplianceUsageEvent will report compliance usage events to posthog.
func (s *Service) emitAccessListReviewComplianceUsageEvent(ctx context.Context, totalAccessLists, accessListsNeedReview int) {
	if s.usageReporter == nil {
		return
	}

	s.usageReporter.AnonymizeAndSubmit(&usagereporter.AccessListReviewComplianceEvent{
		TotalAccessLists:      int32(totalAccessLists),
		AccessListsNeedReview: int32(accessListsNeedReview),
	})
}
