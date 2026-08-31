package oktaservice

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
)

func (s *Service) fetchAllOktaGroups(ctx context.Context, req *oktapb.GetGroupsRequest) ([]*oktaResourceItem, error) {
	oktaClient, err := s.createOktaClient(ctx, req, nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta client")
	}
	fetchFn := func(ctx context.Context) ([]*oktaResourceItem, error) {
		var groups []*oktaResourceItem
		err := oktaClient.IterateGroups(ctx, func(g *okta.Group) error {
			if g.Profile == nil {
				s.logger.DebugContext(ctx, "Skipping missing profile for group", "group_id", g.Id)
				return nil
			}
			groups = append(groups, &oktaResourceItem{
				Name:        g.Profile.Name,
				Description: g.Profile.Description,
			})
			return nil
		})
		return groups, trace.Wrap(err)
	}
	cacheKey := fmt.Sprintf("%s-groups", req.GetOktaOrganizationUrl())
	groups, err := s.cache.Get(ctx, cacheKey, fetchFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return groups, nil
}

func (s *Service) fetchAllOktaApps(ctx context.Context, req *oktapb.GetAppsRequest) ([]*oktaResourceItem, error) {
	oktaClient, err := s.createOktaClient(ctx, req, nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating Okta client")
	}
	fetchFn := func(ctx context.Context) ([]*oktaResourceItem, error) {
		var apps []*oktaResourceItem
		err := oktaClient.IterateApps(ctx, func(a okta.App) error {
			// This type assertion is necessary as okta.App, which is supplied by the Okta go SDK,
			// does not contain all the information that we need to create a types.Application
			// object.
			var oktaApplication *okta.Application
			var ok bool
			if oktaApplication, ok = a.(*okta.Application); !ok {
				s.logger.DebugContext(ctx, "Unable to process Okta application of unknown type")
				return nil
			}
			apps = append(apps, &oktaResourceItem{
				Name: oktaApplication.Label,
			})
			return nil
		})
		return apps, trace.Wrap(err)
	}
	cacheKey := fmt.Sprintf("%s-apps", req.GetOktaOrganizationUrl())
	apps, err := s.cache.Get(ctx, cacheKey, fetchFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return apps, nil
}
