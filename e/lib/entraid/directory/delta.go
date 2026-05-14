package directory

import (
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// isRemoved checks if the delta object signifies
// that the directory object was removed.
// The reason value can either be "changed" or "deleted".
// "changed" means temporarily removed
// "deleted" means permanently removed.
// Both values are treated as "deleted" because the service
// does not handle a notion of soft/hard delete and should
// remove the object from Teleport backend where appropriate.
// https://learn.microsoft.com/en-us/graph/delta-query-overview#resource-representation-in-the-delta-query-response
// https://learn.microsoft.com/en-us/entra/architecture/recover-from-deletions
//
// TODO(sshah): move this helper to /lib/msgraph.
func isRemoved(removed *models.RemovedReason) bool {
	return removed != nil && removed.Reason != nil && (*removed.Reason == "changed" || *removed.Reason == "deleted")
}
