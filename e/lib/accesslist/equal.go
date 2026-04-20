package accesslist

import (
	"github.com/gravitational/teleport/api/types/accesslist"
)

func isReviewChangesAllowed(in accesslist.ReviewChanges) bool {
	return in.MembershipRequirementsChanged == nil &&
		in.ReviewFrequencyChanged == 0 &&
		in.ReviewDayOfMonthChanged == 0
}
