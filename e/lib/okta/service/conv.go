package oktaservice

import oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"

func toGroups(in []*oktaResourceItem) []*oktapb.GetGroupsResponse_Group {
	if in == nil {
		return nil
	}
	out := make([]*oktapb.GetGroupsResponse_Group, 0, len(in))
	for _, v := range in {
		out = append(out, &oktapb.GetGroupsResponse_Group{
			Name:        v.Name,
			Description: v.Description,
		})
	}
	return out
}

func toApps(in []*oktaResourceItem) []*oktapb.GetAppsResponse_App {
	if in == nil {
		return nil
	}
	out := make([]*oktapb.GetAppsResponse_App, 0, len(in))
	for _, v := range in {
		out = append(out, &oktapb.GetAppsResponse_App{
			Name:        v.Name,
			Description: v.Description,
		})
	}
	return out
}
