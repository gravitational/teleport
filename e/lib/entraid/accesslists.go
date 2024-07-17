package entraid

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services"
)

func (r *DirectoryReconciler) reconcileAccessLists(ctx context.Context, usersByEntraID map[entraUniqueID]types.User) error {
	teleportAccessLists, err := listTeleportAccessLists(ctx, r.accessListSvc)
	if err != nil {
		return trace.Wrap(err)
	}
	entraAccessLists, err := listEntraAccessLists(ctx, r.graphClient, r.tenantID, r.defaultOwners)
	if err != nil {
		return trace.Wrap(err)
	}

	alReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher:             matchByLabel[*accesslist.AccessList],
		GetCurrentResources: func() map[string]*accesslist.AccessList { return teleportAccessLists },
		GetNewResources:     func() map[string]*accesslist.AccessList { return entraAccessLists },
		OnCreate: func(ctx context.Context, al *accesslist.AccessList) error {
			_, err := r.accessListSvc.UpsertAccessList(ctx, al)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessList, existing *accesslist.AccessList) error {
			_, err := r.accessListSvc.UpsertAccessList(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, al *accesslist.AccessList) error {
			err := r.accessListSvc.DeleteAccessList(ctx, al.GetName())
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	accessListValues := make([]*accesslist.AccessList, 0, len(teleportAccessLists))
	for _, al := range teleportAccessLists {
		accessListValues = append(accessListValues, al)
	}
	teleportMembers, err := listTeleportAccessListMembers(ctx, r.accessListSvc, accessListValues)
	if err != nil {
		return trace.Wrap(err)
	}

	entraMembers, err := listEntraAccessListMembers(ctx, r.graphClient, usersByEntraID, entraAccessLists)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, src := range teleportMembers {
		if dst, ok := entraMembers[src.GetName()]; ok {
			preserveAccessListMemberMetadata(dst, src)
		}
	}

	memberReconciler, err := services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher:             matchByLabel[*accesslist.AccessListMember],
		GetCurrentResources: func() map[string]*accesslist.AccessListMember { return teleportMembers },
		GetNewResources:     func() map[string]*accesslist.AccessListMember { return entraMembers },
		OnCreate: func(ctx context.Context, m *accesslist.AccessListMember) error {
			_, err := r.accessListSvc.UpsertAccessListMember(ctx, m)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming *accesslist.AccessListMember, existing *accesslist.AccessListMember) error {
			_, err := r.accessListSvc.UpsertAccessListMember(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, m *accesslist.AccessListMember) error {
			err := r.accessListSvc.DeleteAccessListMember(ctx, m.Spec.AccessList, m.Spec.Name)
			// As access lists are reconciled before members,
			// an access list removed from entra can get removed before we try to unassign members.
			// In such cases, simply ignore the "access list not found" error
			if trace.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = trace.NewAggregate(alReconciler.Reconcile(ctx), memberReconciler.Reconcile(ctx))
	if err != nil {
		return trace.Wrap(err)
	}

	r.importedGroups = len(entraAccessLists)
	return nil
}

func listTeleportAccessLists(ctx context.Context, svc accessListAccessPoint) (map[string]*accesslist.AccessList, error) {
	result := map[string]*accesslist.AccessList{}

	var accessLists []*accesslist.AccessList
	var pageToken string
	var err error
	for {
		accessLists, pageToken, err = svc.ListAccessLists(ctx, 0 /* use the default page size*/, pageToken)
		if err != nil {
			return nil, trace.Wrap(err, "listing teleport entra users")
		}

		for _, al := range accessLists {
			if matchByLabel(al) {
				result[al.GetName()] = al
			}
		}

		if pageToken == "" {
			break
		}
	}

	return result, nil
}

func listEntraAccessLists(ctx context.Context, graphClient graphClient, tenantID string, defaultOwners []accesslist.Owner) (map[string]*accesslist.AccessList, error) {
	result := map[string]*accesslist.AccessList{}
	err := graphClient.IterateGroups(ctx, func(g *msgraph.Group) bool {
		al, err := convertGroup(g, tenantID, defaultOwners)
		if err == nil {
			result[al.GetName()] = al
		} else {
			slog.ErrorContext(ctx, "failed to convert Entra ID group to Teleport access list", "error", err)
		}
		return true
	})

	return result, trace.Wrap(err)
}

func listTeleportAccessListMembers(ctx context.Context, svc accessListAccessPoint, als []*accesslist.AccessList) (map[string]*accesslist.AccessListMember, error) {
	result := map[string]*accesslist.AccessListMember{}

	var members []*accesslist.AccessListMember
	var pageToken string
	var err error
	for _, al := range als {
		for {
			members, pageToken, err = svc.ListAccessListMembers(ctx, al.GetName(), 0 /* use the default page size */, pageToken)
			if err != nil {
				return nil, trace.Wrap(err, "listing teleport access list members")
			}
			for _, alm := range members {
				if matchByLabel(alm) {
					result[memberMapKey(alm)] = alm
				}
			}
			if pageToken == "" {
				break
			}
		}
	}

	return result, nil
}

// NB: this enriches Access Lists passed in `als` with their child Access Lists.
func listEntraAccessListMembers(ctx context.Context, graphClient graphClient, entraUsersByID map[entraUniqueID]types.User, als map[string]*accesslist.AccessList) (map[string]*accesslist.AccessListMember, error) {
	result := map[string]*accesslist.AccessListMember{}
	// TODO(justinas): look into batching this if possible.
	for _, al := range als {
		id, ok := al.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			return nil, trace.BadParameter("access list %v missing Entra ID unique ID label", al.GetName())
		}
		err := graphClient.IterateGroupMembers(ctx, id, func(member msgraph.GroupMember) bool {
			alm, err := convertGroupMember(ctx, member, al, entraUsersByID)
			if err != nil {
				var id string
				if member.GetID() != nil {
					id = *member.GetID()
				}
				slog.WarnContext(ctx, "error while converting group member", "member", id, "error", err)
				return false
			}
			if alm == nil {
				slog.WarnContext(ctx, "unsupported group member, skipping")
				return true
			}
			result[memberMapKey(alm)] = alm
			return true
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return result, nil
}

func convertGroup(in *msgraph.Group, tenantID string, defaultOwners []accesslist.Owner) (*accesslist.AccessList, error) {
	if in.DisplayName == nil {
		return nil, trace.BadParameter("expected Entra ID group to have a non-empty display name")
	}
	displayName := *in.DisplayName
	if in.ID == nil {
		return nil, trace.BadParameter("expected Entra ID group to have a non-empty ID")
	}
	id := *in.ID

	out, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName(displayName, id),
		},
		accesslist.Spec{
			Title:  displayName,
			Owners: defaultOwners,
			Grants: accesslist.Grants{
				Traits: trait.Traits{
					eteleport.EntraMemberOfGroupTrait: {id},
				},
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.SetStaticLabels(map[string]string{
		types.EntraTenantIDLabel:    tenantID,
		types.EntraUniqueIDLabel:    id,
		types.EntraDisplayNameLabel: displayName,
	})
	out.SetOrigin(types.OriginEntraID)
	return out, nil
}

// convertGroupMember converts an Entra group member to an AccessListMember.
// Error is returned on unexpected conditions, indicating programmer error (e.g. validation of AccessListMember fails).
// On non fatal errors, e.g. an unsupported member type, a warning is logged and (nil, nil) is returned.
func convertGroupMember(ctx context.Context, in msgraph.GroupMember, al *accesslist.AccessList, entraUsersByID map[entraUniqueID]types.User) (*accesslist.AccessListMember, error) {
	if in.GetID() == nil {
		return nil, trace.BadParameter("expected Entra ID user to have a non-empty unique ID")
	}
	id := *in.GetID()

	switch in.(type) {
	case *msgraph.User:
		teleportUser, ok := entraUsersByID[entraUniqueID(id)]
		if !ok {
			slog.WarnContext(ctx, "no teleport user found for Entra unique ID", "id", id)
			return nil, nil
		}
		alm, err := accesslist.NewAccessListMember(
			header.Metadata{
				Name: teleportUser.GetName(),
			},
			accesslist.AccessListMemberSpec{
				AccessList: al.GetName(),
				Name:       teleportUser.GetName(),
				Joined:     time.Now().UTC(),
				AddedBy:    teleport.UserSystem,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alm.SetOrigin(types.OriginEntraID)
		return alm, nil

	case *msgraph.Group:
		slog.WarnContext(ctx, "entra groups as members of groups are not supported yet", "member", id)
		return nil, nil

	default:
		slog.WarnContext(ctx, "entra group member is not of a supported type: ", "directory_object", in, "type", reflect.TypeOf(in))
		return nil, nil
	}
}

func accessListName(displayName string, id string) string {
	// form a unique name by getting rid of invalid characters and appending the unique ID
	// TODO(justinas): we can shorten this by e.g. hashing the ID and truncating it to a reasonable length
	return resourceNameClean(displayName) + "-" + id
}

var resourceNameCleanRegexp = regexp.MustCompile("[^A-z0-9-.+]+")

// resourceNameClean transforms the given string into one that is a valid name for Teleport resource names.
// It does so by replacing spaces with dashes `-`, and removing any non-alphanumeric characters altogether.
// E.g.:
//   - `Access List #1` is transformed to `access-list-1`
//   - `interns-dev` is kept as is (the name is composed of only valid characters).
func resourceNameClean(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = resourceNameCleanRegexp.ReplaceAllString(s, "")
	return s
}

func preserveAccessListMemberMetadata(dst, src *accesslist.AccessListMember) {
	dst.Spec.Joined = src.Spec.Joined
}

func memberMapKey(member *accesslist.AccessListMember) string {
	return fmt.Sprintf("%s/%s", member.Spec.AccessList, member.GetName())
}
