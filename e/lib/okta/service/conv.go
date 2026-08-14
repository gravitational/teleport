package oktaservice

import (
	"google.golang.org/protobuf/types/known/durationpb"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
)

func toGroups(in []*oktaResourceItem) []*oktapb.GetGroupsResponse_Group {
	if in == nil {
		return nil
	}
	out := make([]*oktapb.GetGroupsResponse_Group, 0, len(in))
	for _, v := range in {
		out = append(out, oktapb.GetGroupsResponse_Group_builder{
			Name:        v.Name,
			Description: v.Description,
		}.Build())
	}
	return out
}

func toApps(in []*oktaResourceItem) []*oktapb.GetAppsResponse_App {
	if in == nil {
		return nil
	}
	out := make([]*oktapb.GetAppsResponse_App, 0, len(in))
	for _, v := range in {
		out = append(out, oktapb.GetAppsResponse_App_builder{
			Name:        v.Name,
			Description: v.Description,
		}.Build())
	}
	return out
}

// durationToString differs from durationpb.Duration.String() in one aspect. It returns empty
// string instead of "0s" if the duration is not set. This is desired because we don't want to set
// "0s" when we use a default value for Okta time_between_sync setting.
func durationToString(d *durationpb.Duration) string {
	if d == nil {
		return ""
	}
	td := d.AsDuration()
	if td == 0 {
		return ""
	}
	return td.String()
}
