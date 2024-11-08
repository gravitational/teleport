package principal

import (
	"context"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	icIter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/lib/services"
)

// Map defines a collection of PrincipalAssignments indexed by their ID
type Map map[services.PrincipalAssignmentID]*identitycenterv1.PrincipalAssignment

// Load loads all of the PrincipalAssignment records from the supplied lister
// into memory and returns them in a map indexed bu their IDs
func Load(ctx context.Context, src icIter.PrincipalAssignmentLister) (Map, error) {
	dst := make(Map)
	for pa, err := range icIter.AllPrincipalAssignments(ctx, src) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dst[GetID(pa)] = pa
	}
	return dst, nil
}
