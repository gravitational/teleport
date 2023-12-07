package accesslist

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// defaultAccessListPageSize is the default page size to be used.
	defaultAccessListPageSize = 100

	// eventMemberBatches is the number of members to emit per event. This will batch member events emitted by this service.
	eventMemberBatches = 50
)

// ignoreFieldsDuringUpsert will be used to ignore fields that are allowed to be modified
// during upsert.
var ignoreFieldsDuringUpsert = []cmp.Option{
	// ID is handled by the backend, so it'll be ignored here.
	cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "MembershipRequires"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "Audit"),
}

type UsersService interface {
	ListUsers(ctx context.Context, pageSize int, nextToken string, withSecrets bool) ([]types.User, string, error)
}

type AuthServer interface {
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	SubmitAccessReview(ctx context.Context, req types.AccessReviewSubmission) (types.AccessRequest, error)
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
}

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// AccessLists is the access list service to use.
	AccessLists services.AccessLists

	// LockGetter is a getter for locks.
	LockGetter services.LockGetter

	// AccessListReviews is the access list reviews service to use.
	AccessListReviews services.AccessListReviews

	// Emitter is the event emitter to use.
	Emitter apievents.Emitter

	// UsageEventsClient is the client for sending usage events metrics.
	UsageEvents UsageEventsClient

	// Clock is the clock.
	Clock clockwork.Clock

	CachedUsersServices UsersService

	// AuthServer implements the minimal auth server interface.
	AuthServer AuthServer
}

// UsageEventsClient is an interface that allows for submitting usage events to Posthog.
type UsageEventsClient interface {
	// SubmitUsageEvent submits an external usage event.
	SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	if c.AccessLists == nil {
		return trace.BadParameter("accesslists service is missing")
	}

	if c.LockGetter == nil {
		return trace.BadParameter("lockgetter service is missing")
	}

	if c.AccessListReviews == nil {
		return trace.BadParameter("accesslistreviews service is missing")
	}

	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}

	if c.CachedUsersServices == nil {
		return trace.BadParameter("CachedUsersServices is missing")
	}

	if modules.GetModules().Features().Cloud {
		if c.UsageEvents == nil {
			return trace.BadParameter("missing usage events")
		}
	} else {
		c.UsageEvents = nil
	}

	if c.AuthServer == nil {
		return trace.BadParameter("auth server is missing")
	}

	if c.Logger == nil {
		c.Logger = logrus.New().WithField(trace.Component, "access_list_crud_service")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

type Service struct {
	accesslistv1.UnimplementedAccessListServiceServer

	log               logrus.FieldLogger
	authorizer        authz.Authorizer
	accessLists       services.AccessLists
	membershipChecker *services.AccessListMembershipChecker
	accessListReviews services.AccessListReviews
	usageEvents       UsageEventsClient
	emitter           apievents.Emitter
	clock             clockwork.Clock
	cachedUsers       UsersService
	authServer        AuthServer

	// When not set, this will use the default page size for ListUsers.
	userPageSize int
}

// NewService creates a new Access List gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:         cfg.Logger,
		authorizer:  cfg.Authorizer,
		accessLists: cfg.AccessLists,
		membershipChecker: services.NewAccessListMembershipChecker(
			cfg.Clock, cfg.AccessLists, cfg.LockGetter),
		accessListReviews: cfg.AccessListReviews,
		usageEvents:       cfg.UsageEvents,
		emitter:           cfg.Emitter,
		clock:             cfg.Clock,
		cachedUsers:       cfg.CachedUsersServices,
		authServer:        cfg.AuthServer,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (s *Service) GetAccessLists(ctx context.Context, _ *accesslistv1.GetAccessListsRequest) (*accesslistv1.GetAccessListsResponse, error) {
	// We don't return these errors right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	results, getErr := s.accessLists.GetAccessLists(ctx)
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)

	var err error
	results, err = s.filterResults(ctx, results, false, getErr, authErr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.GetAccessListsResponse{
		AccessLists: accessLists,
	}, nil
}

// ListAccessLists returns a paginated list of all access lists.
func (s *Service) ListAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsRequest) (*accesslistv1.ListAccessListsResponse, error) {
	pageSize := int(req.PageSize)

	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}
	// We don't return the auth error right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)

	var results []*accesslist.AccessList
	nextToken := req.NextToken
	for {
		var page []*accesslist.AccessList
		var getErr error
		page, nextToken, getErr = s.accessLists.ListAccessLists(ctx, 0 /* default page size in backend */, nextToken)

		var err error
		page, err = s.filterResults(ctx, page, true, getErr, authErr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		results = append(results, page...)
		if len(results) >= (pageSize+1) || nextToken == "" {
			break
		}
	}

	// Truncate the results.
	if len(results) > pageSize {
		nextToken = backend.GetPaginationKey(results[pageSize])
		results = results[:pageSize]
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.ListAccessListsResponse{
		AccessLists: accessLists,
		NextToken:   nextToken,
	}, nil
}

// filterResults will return the following:
// * If the user has RBAC access to the access lists (authErr == nil), the access lists will be returned as is.
// * If the user owns any access lists, these will be returned with membership information retained.
// * IF the user is a member of any access lists, these will be returned with membership information stripped.
func (s *Service) filterResults(ctx context.Context, results []*accesslist.AccessList, isPaginated bool, getErr, authErr error) ([]*accesslist.AccessList, error) {
	if getErr != nil && authErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, trace.Wrap(authErr)
	} else if authErr != nil {
		// We successfully got the access lists but had an issue authorizing. Check to see if the user is an
		// owner for any of these lists.
		var filteredResults []*accesslist.AccessList

		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := authCtx.Identity.GetIdentity()
		for _, result := range results {
			if err := services.IsAccessListOwner(identity, result); err == nil {
				filteredResults = append(filteredResults, result)
			} else if err := s.membershipChecker.IsAccessListMember(ctx, identity, result); err == nil {
				filteredResults = append(filteredResults, result)
			}
		}

		results = filteredResults
	}

	// The user owns no access lists and received an auth err earlier. Also, we're not looking
	// at paginated lists.
	if len(results) == 0 && isPaginated {
		return nil, trace.Wrap(authErr)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	return results, nil
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	result, getErr := s.accessLists.GetAccessList(ctx, req.GetName())

	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead)
	if getErr != nil && authErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, trace.Wrap(authErr)
	} else if authErr != nil {
		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := authCtx.Identity.GetIdentity()
		// Check if the user's an owner. If not, then we'll check if the user is a member. If neither are
		// true, we'll return the original auth error.
		if ownerErr := services.IsAccessListOwner(identity, result); ownerErr != nil {
			if memberErr := s.membershipChecker.IsAccessListMember(ctx, identity, result); memberErr != nil {
				return nil, trace.Wrap(authErr)
			}
		}
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	// Get a list of all users, to compute eligibility for owners.
	users, err := s.getAllUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	// Go through owners and determine eligibility.
	updatedOwners := make([]accesslist.Owner, len(result.GetOwners()))
	for i, owner := range result.GetOwners() {
		ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
			userLookup: userLookup,
			username:   owner.Name,
			expires:    time.Time{}, // owners don't have expiry's
			clock:      s.clock,
			requires:   result.GetOwnershipRequires(),
		})

		owner.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
		updatedOwners[i] = owner
	}
	result.SetOwners(updatedOwners)

	return conv.ToProto(result), nil
}

// getAllUsers returns all users known to Teleport.
func (s *Service) getAllUsers(ctx context.Context) ([]types.User, error) {
	var users []types.User
	var nextToken string
	for {
		var page []types.User
		var err error
		page, nextToken, err = s.cachedUsers.ListUsers(ctx, s.userPageSize, nextToken, false /* without secrets */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		users = append(users, page...)

		if nextToken == "" {
			break
		}
	}

	return users, nil
}

// GetAccessListsToReview will return access lists that need to be reviewed by the current user.
func (s *Service) GetAccessListsToReview(ctx context.Context, req *accesslistv1.GetAccessListsToReviewRequest) (*accesslistv1.GetAccessListsToReviewResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &accesslistv1.GetAccessListsToReviewResponse{}
	var nextToken string
	now := s.clock.Now()

	for {
		var page []*accesslist.AccessList
		var err error
		page, nextToken, err = s.accessLists.ListAccessLists(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, accessList := range page {
			if needsReviewBy(authCtx.Identity.GetIdentity(), accessList, now) {
				resp.AccessLists = append(resp.AccessLists, conv.ToProto(accessList))
			}
		}

		if nextToken == "" {
			break
		}
	}

	return resp, nil
}

// needsReviewBy returns true if the access list should be reviewed by the user.
func needsReviewBy(identity tlsca.Identity, accessList *accesslist.AccessList, now time.Time) bool {
	return services.IsAccessListOwner(identity, accessList) == nil && accessList.Spec.Audit.NextAuditDate.Sub(now) <= accessList.Spec.Audit.Notifications.Start
}

// UpsertAccessList creates or updates an access list resource.
func (s *Service) UpsertAccessList(ctx context.Context, req *accesslistv1.UpsertAccessListRequest) (*accesslistv1.AccessList, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, updated, upsertErr := s.upsertAccessList(ctx, authCtx, req)

	var accessListName string
	if req != nil && req.AccessList != nil && req.AccessList.Header != nil && req.AccessList.Header.Metadata != nil {
		accessListName = req.AccessList.Header.Metadata.Name
	}

	s.emitUpsertAccessListEvent(ctx, authCtx.Identity.GetIdentity().Username, updated, accessListName, upsertErr)

	if upsertErr == nil {
		s.emitUpsertAccessListUsageEvent(ctx, updated, accessListName)
	}

	return resp, trace.Wrap(upsertErr)
}

// upsertAccessList is a helper for upserting the access list that returns the response, whether this was an update request, and an error.
func (s *Service) upsertAccessList(ctx context.Context, authCtx *authz.Context, req *accesslistv1.UpsertAccessListRequest) (resp *accesslistv1.AccessList, updated bool, err error) {
	oldAccessList, getErr := s.accessLists.GetAccessList(ctx, req.GetAccessList().GetHeader().Metadata.Name)
	if oldAccessList != nil {
		updated = true
	}

	newAccessList, err := conv.FromProto(req.GetAccessList())
	if err != nil {
		return nil, updated, trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}

	_, authErr := authz.AuthorizeContextWithVerbs(ctx, s.log, authCtx, true, ruleCtx, types.KindAccessList, types.VerbCreate, types.VerbUpdate)
	// Check if the user is the owner of the list.
	if authErr != nil && getErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, updated, trace.Wrap(authErr)
	} else if authErr != nil {
		identity := authCtx.Identity.GetIdentity()
		if ownerErr := services.IsAccessListOwner(identity, oldAccessList); ownerErr != nil {
			// The user does not own this list, so return the original auth error.
			return nil, updated, trace.Wrap(authErr)
		}

		// Owners are only allowed to modify members, membership_requires, and audit interval.
		if !cmp.Equal(newAccessList, oldAccessList, ignoreFieldsDuringUpsert...) {
			return nil, updated, trace.AccessDenied("owners can only modify audit, members, and membership_requires")
		}
	}

	if getErr != nil && !trace.IsNotFound(getErr) {
		return nil, updated, trace.Wrap(getErr)
	}

	responseAccessList, err := s.accessLists.UpsertAccessList(ctx, newAccessList)
	if err != nil {
		return nil, updated, trace.Wrap(err)
	}

	return conv.ToProto(responseAccessList), updated, nil
}

// emitUpsertAccessListEvent will emit the create/update event for the access list.
func (s *Service) emitUpsertAccessListEvent(ctx context.Context, username string, updated bool, accessListName string, upsertErr error) {
	var errorMsg string
	if upsertErr != nil {
		errorMsg = upsertErr.Error()
	}

	resourceMetadata := apievents.ResourceMetadata{
		Name:      accessListName,
		UpdatedBy: username,
	}
	status := apievents.Status{
		Success: upsertErr == nil,
		Error:   errorMsg,
	}
	var event apievents.AuditEvent
	if updated {
		event = &apievents.AccessListUpdate{
			Metadata: apievents.Metadata{
				Type: events.AccessListUpdateEvent,
				Code: events.AccessListUpdateSuccessCode,
			},
			ResourceMetadata: resourceMetadata,
			Status:           status,
		}
		if upsertErr != nil {
			event.SetCode(events.AccessListUpdateFailureCode)
		}
	} else {
		event = &apievents.AccessListCreate{
			Metadata: apievents.Metadata{
				Type: events.AccessListCreateEvent,
				Code: events.AccessListCreateSuccessCode,
			},
			ResourceMetadata: resourceMetadata,
			Status:           status,
		}
		if upsertErr != nil {
			event.SetCode(events.AccessListCreateFailureCode)
		}
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit access list create/update event: %v", event)
	}
}

// emitUpsertAccessListUsageEvent will emit a posthog event for upserting an access list.
func (s *Service) emitUpsertAccessListUsageEvent(ctx context.Context, updated bool, accessListName string) {
	if s.usageEvents == nil {
		return
	}

	var event *usageeventsv1.UsageEventOneOf
	if updated {
		event = &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListUpdate{
				AccessListUpdate: &usageeventsv1.AccessListUpdate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		}
	} else {
		event = &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListCreate{
				AccessListCreate: &usageeventsv1.AccessListCreate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		}
	}
	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{Event: event}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list create/update usage event")
	}
}

// DeleteAccessList removes the specified access list resource.
func (s *Service) DeleteAccessList(ctx context.Context, req *accesslistv1.DeleteAccessListRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, deleteErr := s.deleteAccessList(ctx, authCtx, req)

	s.emitDeleteAccessListEvent(ctx, authCtx, req.Name, deleteErr)

	if deleteErr == nil {
		s.emitDeleteAccessListUsageEvent(ctx, req.Name)
	}

	return resp, trace.Wrap(deleteErr)
}

// deleteAccessList is a helper for deleting the access list that returns the response and an error.
func (s *Service) deleteAccessList(ctx context.Context, authCtx *authz.Context, req *accesslistv1.DeleteAccessListRequest) (*emptypb.Empty, error) {
	ruleCtx := &services.Context{
		User: authCtx.User,
	}

	_, err := authz.AuthorizeContextWithVerbs(ctx, s.log, authCtx, true, ruleCtx, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.accessLists.DeleteAccessList(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// emitDeleteAccessListEvent will emit the delete event for the access list.
func (s *Service) emitDeleteAccessListEvent(ctx context.Context, authCtx *authz.Context, accessListName string, deleteErr error) {
	var errorMsg string
	eventCode := events.AccessListDeleteSuccessCode
	if deleteErr != nil {
		eventCode = events.AccessListDeleteFailureCode
		errorMsg = deleteErr.Error()
	}

	event := &apievents.AccessListDelete{
		Metadata: apievents.Metadata{
			Type: events.AccessListDeleteEvent,
			Code: eventCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      accessListName,
			UpdatedBy: authCtx.User.GetName(),
		},
		Status: apievents.Status{
			Success: deleteErr == nil,
			Error:   errorMsg,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit access list delete event: %v", event)
	}
}

// emitDeleteAccessListUsageEvent will emit a posthog event for deleting an access list.
func (s *Service) emitDeleteAccessListUsageEvent(ctx context.Context, accessListName string) {
	if s.usageEvents == nil {
		return
	}

	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListDelete{
				AccessListDelete: &usageeventsv1.AccessListDelete{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		},
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list delete usage event")
	}
}

// DeleteAllAccessLists removes all access lists.
func (s *Service) DeleteAllAccessLists(ctx context.Context, _ *accesslistv1.DeleteAllAccessListsRequest) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("DeleteAllAccessLists not supported in the gRPC server")
}

// ListAccessListMembers returns a paginated list of all access list members.
func (s *Service) ListAccessListMembers(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	retrievedAccessList, err := s.authOrIsOwnerWithAccessList(ctx, req.AccessList, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := s.accessLists.ListAccessListMembers(ctx, req.AccessList, int(req.PageSize), req.PageToken)
	switch {
	case err == nil:
		break

	case errors.Is(err, services.ImplicitAccessListError{}):
		return s.listImplicitAccessListMembers(ctx, req.AccessList, int(req.PageSize), req.PageToken)

	default:
		return nil, trace.Wrap(err)
	}

	// Get a list of all users, to compute eligibility for members.
	users, err := s.getAllUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	members := make([]*accesslistv1.Member, len(results))
	for i, r := range results {
		ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
			userLookup: userLookup,
			username:   r.GetName(),
			expires:    r.Spec.Expires,
			clock:      s.clock,
			requires:   retrievedAccessList.GetMembershipRequires(),
		})
		r.Spec.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
		members[i] = conv.ToMemberProto(r)
	}

	return &accesslistv1.ListAccessListMembersResponse{
		Members:       members,
		NextPageToken: nextToken,
	}, nil
}

func generateEphemeralMember(accessList *accesslist.AccessList, user types.User, clock clockwork.Clock) (*accesslistv1.Member, error) {
	m, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: fmt.Sprintf("%s%c%s", accessList.GetName(), backend.Separator, user.GetName()),
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList.GetName(),
			Membership: accesslist.InclusionImplicit,
			Name:       user.GetName(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conv.ToMemberProto(m), nil
}

// listImplicitAccessListMembers generates a snapshot of the current member
// list for an AccessList with implicit membership. Note that this method
// performs no authorization. This must be handled by the caller.
func (s *Service) listImplicitAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (*accesslistv1.ListAccessListMembersResponse, error) {
	if pageSize <= 0 {
		pageSize = s.userPageSize
	}

	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var members []*accesslistv1.Member
	for {
		var candidates []types.User

		// Query for *at most* enough candidate users such that, even if all
		// of them are identified as members, we don't blow the page size
		// budget that the caller has set us
		candidatePageSize := min(pageSize-len(members), s.userPageSize)
		candidates, pageToken, err = s.cachedUsers.ListUsers(ctx, candidatePageSize, pageToken, false /* without secrets */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Filter out all of the non-members, copying the identified members
		// into the output page buffer
		for _, candidate := range candidates {
			isMember := services.UserMeetsRequirements(tlsca.Identity{
				Groups: candidate.GetRoles(),
				Traits: candidate.GetTraits(),
			}, accessList.Spec.MembershipRequires)

			if isMember {
				member, err := generateEphemeralMember(accessList, candidate, s.clock)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				members = append(members, member)
			}
		}

		// if there are no more users to iterate over...
		if pageToken == "" {
			break
		}

		// if we have hit the maximum size of the page we want...
		if len(members) == pageSize {
			break
		}
	}

	return &accesslistv1.ListAccessListMembersResponse{
		Members:       members,
		NextPageToken: pageToken,
	}, nil
}

// GetAccessListMember returns the specified access list member resource.
func (s *Service) GetAccessListMember(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslistv1.Member, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := s.accessLists.GetAccessListMember(ctx, req.AccessList, req.MemberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), nil
}

// UpsertAccessListMember creates or updates an access list member resource.
func (s *Service) UpsertAccessListMember(ctx context.Context, req *accesslistv1.UpsertAccessListMemberRequest) (*accesslistv1.Member, error) {
	if err := s.authOrIsOwner(ctx, req.Member.Spec.AccessList, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.Member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	resp, accessListName, updated, upsertErr := s.upsertAccessListMember(ctx, username, member)

	var joinTime time.Time
	if resp != nil {
		joinTime = resp.Spec.Joined.AsTime()
	}

	s.emitUpsertAccessListMemberEvent(ctx, username, updated, accessListName, upsertErr,
		accessListMembersForEvent(joinTime, time.Time{}, accessListMemberProtoToMemberEventMetadata(req.Member))...)

	if upsertErr == nil {
		s.emitUpsertAccessListMemberUsageEvent(ctx, updated, accessListName)
	}

	return resp, trace.Wrap(upsertErr)
}

// upsertAccessListMember is a helper for creating or updating access list members that returns the response, whether this was an update, and an error.
func (s *Service) upsertAccessListMember(ctx context.Context, username string,
	member *accesslist.AccessListMember) (resultProto *accesslistv1.Member, accessListName string, updated bool, err error) {
	updated = false

	// If the user didn't exist before, make sure the current user is recorded as the user that added it.
	if oldMember, err := s.accessLists.GetAccessListMember(ctx, member.Spec.AccessList, member.GetName()); trace.IsNotFound(err) {
		member.Spec.AddedBy = username
		member.Spec.Joined = s.clock.Now()
	} else if err == nil {
		updated = true
		// If the user already existed, use the old added by, reason, and joined.
		member.Spec.AddedBy = oldMember.Spec.AddedBy
		member.Spec.Joined = oldMember.Spec.Joined
	} else {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	result, err := s.accessLists.UpsertAccessListMember(ctx, member)
	if err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), member.Spec.AccessList, updated, nil
}

// emitUpsertAccessListMemberEvent will emit the create/update event for the access list member.
func (s *Service) emitUpsertAccessListMemberEvent(ctx context.Context, username string, updated bool, accessListName string, upsertErr error, members ...*apievents.AccessListMember) {
	var errorMsg string
	if upsertErr != nil {
		errorMsg = upsertErr.Error()
	}

	resourceMetadata := apievents.ResourceMetadata{
		UpdatedBy: username,
	}
	status := apievents.Status{
		Success: upsertErr == nil,
		Error:   errorMsg,
	}

	for _, batch := range batchAccessListMemberMetadata(accessListName, members) {
		var event apievents.AuditEvent
		if updated {
			event = &apievents.AccessListMemberUpdate{
				Metadata: apievents.Metadata{
					Type: events.AccessListMemberUpdateEvent,
					Code: events.AccessListMemberUpdateSuccessCode,
				},
				ResourceMetadata:         resourceMetadata,
				AccessListMemberMetadata: batch,
				Status:                   status,
			}
			if upsertErr != nil {
				event.SetCode(events.AccessListMemberUpdateFailureCode)
			}
		} else {
			event = &apievents.AccessListMemberCreate{
				Metadata: apievents.Metadata{
					Type: events.AccessListMemberCreateEvent,
					Code: events.AccessListMemberCreateSuccessCode,
				},
				ResourceMetadata:         resourceMetadata,
				AccessListMemberMetadata: batch,
				Status:                   status,
			}
			if upsertErr != nil {
				event.SetCode(events.AccessListMemberCreateFailureCode)
			}
		}

		if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
			s.log.WithError(emitErr).Warnf("Failed to emit access list member create/update event: %v", event)
		}
	}
}

func (s *Service) emitUpsertAccessListMemberUsageEvent(ctx context.Context, updated bool, accessListName string) {
	if s.usageEvents == nil {
		return
	}

	var event *usageeventsv1.UsageEventOneOf
	if updated {
		event = &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListMemberUpdate{
				AccessListMemberUpdate: &usageeventsv1.AccessListMemberUpdate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		}
	} else {
		event = &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListMemberCreate{
				AccessListMemberCreate: &usageeventsv1.AccessListMemberCreate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		}
	}
	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{Event: event}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list member create/update usage event")
	}
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (s *Service) DeleteAccessListMember(ctx context.Context, req *accesslistv1.DeleteAccessListMemberRequest) (*emptypb.Empty, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	resp, deleteErr := s.deleteAccessListMember(ctx, req)

	s.emitDeleteAccessListMemberEvent(ctx, username, req.AccessList, deleteErr,
		accessListMembersForEvent(time.Time{}, s.clock.Now(), &memberEventMetadata{name: req.MemberName})...)

	if deleteErr == nil {
		s.emitDeleteAccessListMemberUsageEvent(ctx, req.AccessList)
	}

	return resp, trace.Wrap(deleteErr)
}

// deleteAccessListMember is a helper for deleting access list members that returns the response and an error.
func (s *Service) deleteAccessListMember(ctx context.Context, req *accesslistv1.DeleteAccessListMemberRequest) (*emptypb.Empty, error) {
	err := s.accessLists.DeleteAccessListMember(ctx, req.AccessList, req.MemberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// emitDeleteAccessListMemberEvent will emit the delete event for the access list member.
func (s *Service) emitDeleteAccessListMemberEvent(ctx context.Context, username string, accessListName string, deleteErr error, members ...*apievents.AccessListMember) {
	var errorMsg string
	eventCode := events.AccessListMemberDeleteSuccessCode
	if deleteErr != nil {
		eventCode = events.AccessListMemberDeleteFailureCode
		errorMsg = deleteErr.Error()
	}

	for _, batch := range batchAccessListMemberMetadata(accessListName, members) {
		event := &apievents.AccessListMemberDelete{
			Metadata: apievents.Metadata{
				Type: events.AccessListMemberDeleteEvent,
				Code: eventCode,
			},
			ResourceMetadata: apievents.ResourceMetadata{
				UpdatedBy: username,
			},
			AccessListMemberMetadata: batch,
			Status: apievents.Status{
				Success: deleteErr == nil,
				Error:   errorMsg,
			},
		}

		if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
			s.log.WithError(emitErr).Warnf("Failed to emit access list delete member event: %v", event)
		}
	}
}

// emitDeleteAccessListMemberUsageEvent will emit a posthog event for deleting an access list member.
func (s *Service) emitDeleteAccessListMemberUsageEvent(ctx context.Context, accessListName string) {
	if s.usageEvents == nil {
		return
	}

	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListMemberDelete{
				AccessListMemberDelete: &usageeventsv1.AccessListMemberDelete{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
				},
			},
		},
	}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list delete usage event")
	}
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list (without deleting the access list itself).
func (s *Service) DeleteAllAccessListMembersForAccessList(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) (*emptypb.Empty, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	resp, deleteErr := s.deleteAllAccessListMembersForAccessList(ctx, req)

	s.emitDeleteAllAccessListMembersForAccessListEvent(ctx, username, req.AccessList, deleteErr)

	return resp, trace.Wrap(deleteErr)
}

// deleteAllAccessListMembersForAccessList is a helper for deleting all access list members for an access list that returns the response and an error.
func (s *Service) deleteAllAccessListMembersForAccessList(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) (*emptypb.Empty, error) {
	err := s.accessLists.DeleteAllAccessListMembersForAccessList(ctx, req.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// emitDeleteAllAccessListMembersForAccessListEvent will emit the event for deleting all access list members from an access list.
func (s *Service) emitDeleteAllAccessListMembersForAccessListEvent(ctx context.Context, username string, accessListName string, deleteErr error) {
	var errorMsg string
	eventCode := events.AccessListMemberDeleteAllForAccessListSuccessCode
	if deleteErr != nil {
		eventCode = events.AccessListMemberDeleteAllForAccessListFailureCode
		errorMsg = deleteErr.Error()
	}

	event := &apievents.AccessListMemberDeleteAllForAccessList{
		Metadata: apievents.Metadata{
			Type: events.AccessListMemberDeleteAllForAccessListEvent,
			Code: eventCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			UpdatedBy: username,
		},
		AccessListMemberMetadata: apievents.AccessListMemberMetadata{
			AccessListName: accessListName,
		},
		Status: apievents.Status{
			Success: deleteErr == nil,
			Error:   errorMsg,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit access list delete event: %v", event)
	}
}

// DeleteAllAccessListMembers hard deletes all access list members for all access lists (without deleting the access lists themselves).
func (s *Service) DeleteAllAccessListMembers(_ context.Context, _ *accesslistv1.DeleteAllAccessListMembersRequest) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("DeleteAllAccessListMembers not supported in the gRPC service")
}

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (s *Service) UpsertAccessListWithMembers(ctx context.Context, req *accesslistv1.UpsertAccessListWithMembersRequest) (*accesslistv1.UpsertAccessListWithMembersResponse, error) {
	accessListName := req.AccessList.GetHeader().Metadata.Name
	// Check if the caller is allowed to make any changes to the access list or members.
	if err := s.authOrIsOwner(ctx, accessListName, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	resp, updated, modifiedMembers, upsertErr := s.upsertAccessListWithMembers(ctx, req)

	s.emitUpsertAccessListEvent(ctx, username, updated, accessListName, upsertErr)

	if upsertErr == nil {
		s.emitUpsertAccessListUsageEvent(ctx, updated, accessListName)
	}

	if modifiedMembers != nil {
		if len(modifiedMembers.created) > 0 {
			s.emitUpsertAccessListMemberEvent(ctx, username, false, accessListName, upsertErr,
				accessListMembersForEvent(s.clock.Now(), time.Time{}, accessListMembersToMemberEventMetadata(modifiedMembers.created)...)...,
			)

			if upsertErr == nil {
				for i := 0; i < len(modifiedMembers.created); i++ {
					s.emitUpsertAccessListMemberUsageEvent(ctx, false, accessListName)
				}
			}
		}
		if len(modifiedMembers.updated) > 0 {
			s.emitUpsertAccessListMemberEvent(ctx, username, true, accessListName, upsertErr,
				accessListMembersForEvent(s.clock.Now(), time.Time{}, accessListMembersToMemberEventMetadata(modifiedMembers.updated)...)...,
			)

			if upsertErr == nil {
				for i := 0; i < len(modifiedMembers.updated); i++ {
					s.emitUpsertAccessListMemberUsageEvent(ctx, true, accessListName)
				}
			}
		}
		if len(modifiedMembers.deleted) > 0 {
			s.emitDeleteAccessListMemberEvent(ctx, username, accessListName, upsertErr,
				accessListMembersForEvent(time.Time{}, s.clock.Now(), accessListMembersToMemberEventMetadata(modifiedMembers.deleted)...)...,
			)

			if upsertErr == nil {
				for i := 0; i < len(modifiedMembers.deleted); i++ {
					s.emitDeleteAccessListMemberUsageEvent(ctx, accessListName)
				}
			}
		}
	}

	// Return the updated access list and members.
	return resp, trace.Wrap(upsertErr)
}

// modifiedMembers will be used to house the exact modifications made to the members to emit and event later.
type modifiedMembers struct {
	created []*accesslist.AccessListMember
	updated []*accesslist.AccessListMember
	deleted []*accesslist.AccessListMember
}

// upsertAccessListWithMembers is a helper for upserting an access list with members that returns the response, whether the access list was updated, the modified members, and an error.
func (s *Service) upsertAccessListWithMembers(ctx context.Context,
	req *accesslistv1.UpsertAccessListWithMembersRequest) (*accesslistv1.UpsertAccessListWithMembersResponse, bool, *modifiedMembers, error) {
	var updated bool

	// Determine if we're have an old access list, used for emitting events.
	oldAccessList, err := s.accessLists.GetAccessList(ctx, req.GetAccessList().GetHeader().Metadata.Name)
	if err != nil && !trace.IsNotFound(err) {
		return nil, false, nil, trace.Wrap(err)
	} else if oldAccessList != nil {
		updated = true
	}

	// Get the old members here, also used for emitting events. On error, we won't fail here.
	oldMembers, err := s.getAccessListMemberMap(ctx, req.AccessList.Header.Metadata.GetName())
	if err != nil {
		return nil, updated, nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(req.AccessList)
	if err != nil {
		return nil, updated, nil, trace.Wrap(err)
	}

	// Convert members
	members := make([]*accesslist.AccessListMember, 0, len(req.Members))
	for _, member := range req.Members {
		m, err := conv.FromMemberProto(member)
		if err != nil {
			return nil, updated, nil, trace.Wrap(err)
		}
		members = append(members, m)
	}

	// Call the API.
	updatedAccessList, updatedMembers, err := s.accessLists.UpsertAccessListWithMembers(ctx, accessList, members)
	if err != nil {
		return nil, updated, nil, trace.Wrap(err)
	}

	// Figure out the member modifications for event emitting.
	modified := getModifiedMembers(oldMembers, updatedMembers)

	// Convert members back to proto.
	updatedProtoMembers := make([]*accesslistv1.Member, 0, len(req.Members))
	for _, member := range updatedMembers {
		updatedProtoMembers = append(updatedProtoMembers, conv.ToMemberProto(member))
	}

	// Return the updated access list and members.
	return &accesslistv1.UpsertAccessListWithMembersResponse{
		AccessList: conv.ToProto(updatedAccessList),
		Members:    updatedProtoMembers,
	}, updated, modified, nil
}

// getAccessListMemberMap will return all members for an access list or nil on error. Used for emit events.
func (s *Service) getAccessListMemberMap(ctx context.Context, accessListName string) (map[string]*accesslist.AccessListMember, error) {
	members := map[string]*accesslist.AccessListMember{}
	var pageToken string
	for {
		var page []*accesslist.AccessListMember
		var err error
		page, pageToken, err = s.accessLists.ListAccessListMembers(ctx, accessListName, 0, pageToken)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			break
		}

		for _, member := range page {
			members[member.GetName()] = member
		}
		if pageToken == "" {
			break
		}
	}

	return members, nil
}

// getModifiedMembers will get the modified members of the access list by comparing to the given old member map. If the old member
// map is nil, modified members will be nil.
func getModifiedMembers(oldMembers map[string]*accesslist.AccessListMember, updatedMembers []*accesslist.AccessListMember) *modifiedMembers {
	if oldMembers == nil {
		return nil
	}

	modified := &modifiedMembers{}
	for _, member := range updatedMembers {
		memberName := member.GetName()
		if _, ok := oldMembers[memberName]; ok {
			modified.updated = append(modified.updated, member)
			delete(oldMembers, memberName)
		} else {
			modified.created = append(modified.created, member)
		}
	}

	for _, oldMember := range oldMembers {
		modified.deleted = append(modified.deleted, oldMember)
	}

	return modified
}

// hasAccessListRBAC tests if the user has RBAC access to access lists.
func (s *Service) hasAccessListRBAC(ctx context.Context, authCtx *authz.Context, verbs ...string) bool {
	ruleCtx := &services.Context{
		User: authCtx.User,
	}
	_, authErr := authz.AuthorizeContextWithVerbs(ctx, s.log, authCtx, true, ruleCtx, types.KindAccessList, verbs...)
	if authErr != nil {
		s.log.WithError(authErr).Debug("hasAccessListRBAC had error")
	}

	return authErr == nil
}

// isOwnerOfAccessList checks if this user owns this access list.
func (s *Service) isOwnerOfAccessList(ctx context.Context, authCtx *authz.Context, accessList *accesslist.AccessList) error {
	identity := authCtx.Identity.GetIdentity()
	if err := services.IsAccessListOwner(identity, accessList); err != nil {
		s.log.WithError(err).Debug("isOwnerOfAccessList returned error")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	return nil
}

// AccessRequestPromote promotes an access request to an access list.
func (s *Service) AccessRequestPromote(ctx context.Context, req *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error) {
	accessList, err := s.authOrIsOwnerWithAccessList(ctx, req.AccessListName, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	accessReviewSubmission := types.AccessReviewSubmission{
		RequestID: req.RequestId,
		Review: types.AccessReview{
			ProposedState: types.RequestState_PROMOTED,
			AccessList: &types.PromotedAccessList{
				Name:  req.AccessListName,
				Title: accessList.Spec.Title,
			},
			Reason: req.Reason,
		},
	}

	// review author defaults to username of caller.
	if accessReviewSubmission.Review.Author == "" {
		accessReviewSubmission.Review.Author = authCtx.User.GetName()
	}

	if err := auth.AuthorizeAccessReviewRequest(*authCtx, accessReviewSubmission); err != nil {
		return nil, trace.Wrap(err)
	}

	accessReqs, err := s.authServer.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: req.RequestId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(accessReqs) != 1 {
		return nil, trace.NotFound("access request not found")
	}

	accessReq := accessReqs[0]

	allowedPromotions, err := s.authServer.GetAccessRequestAllowedPromotions(ctx, accessReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if the access list can be used for promotion.
	if !slices.ContainsFunc(allowedPromotions.Promotions, func(p *types.AccessRequestAllowedPromotion) bool {
		return p.AccessListName == req.AccessListName
	}) {
		return nil, trace.AccessDenied("access request cannot be promoted to requested access list")
	}

	memberName := accessReq.GetUser()

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	_, _, _, err = s.upsertAccessListMember(ctx, username, &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Kind:    types.KindAccessListMember,
			Version: types.V3,
			Metadata: header.Metadata{
				Name: memberName,
			},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList: req.AccessListName,
			Name:       memberName,
			AddedBy:    authCtx.User.GetName(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	promotedAccessReq, err := s.authServer.SubmitAccessReview(ctx, accessReviewSubmission)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessRequest, ok := promotedAccessReq.(*types.AccessRequestV3)
	if !ok {
		err = trace.BadParameter("unexpected access request type %T", req)
		return nil, trace.Wrap(err)
	}

	return &accesslistv1.AccessRequestPromoteResponse{
		AccessRequest: accessRequest,
	}, nil
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (s *Service) ListAccessListReviews(ctx context.Context, req *accesslistv1.ListAccessListReviewsRequest) (*accesslistv1.ListAccessListReviewsResponse, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	reviews, nextToken, err := s.accessListReviews.ListAccessListReviews(ctx, req.AccessList, int(req.PageSize), req.NextToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &accesslistv1.ListAccessListReviewsResponse{
		Reviews:   make([]*accesslistv1.Review, len(reviews)),
		NextToken: nextToken,
	}

	for i, review := range reviews {
		resp.Reviews[i] = conv.ToReviewProto(review)
	}

	return resp, nil
}

// CreateAccessListReview will create a new review for an access list. It will also modify the original access list
// and its members depending on the details of the review.
func (s *Service) CreateAccessListReview(ctx context.Context, req *accesslistv1.CreateAccessListReviewRequest) (*accesslistv1.CreateAccessListReviewResponse, error) {
	review, err := conv.FromReviewProto(req.Review)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authOrIsOwner(ctx, review.Spec.AccessList, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := user.GetIdentity().Username

	resp, updatedReview, createErr := s.createAccessListReview(ctx, review, username)

	s.emitCreateAccessListReview(ctx, username, updatedReview, createErr)

	return resp, trace.Wrap(createErr)
}

// createAccessListReview is a helper for creating the access list review that returns the response and an error.
func (s *Service) createAccessListReview(ctx context.Context, review *accesslist.Review, username string) (*accesslistv1.CreateAccessListReviewResponse, *accesslist.Review, error) {
	// Make sure the reviewers reflect the current user and the review date is recorded as now.
	review.Spec.Reviewers = []string{username}
	review.Spec.ReviewDate = s.clock.Now()

	updatedReview, nextAuditDate, err := s.accessListReviews.CreateAccessListReview(ctx, review)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &accesslistv1.CreateAccessListReviewResponse{
		ReviewName:    updatedReview.GetName(),
		NextAuditDate: timestamppb.New(nextAuditDate),
	}, updatedReview, nil
}

// emitCreateAccessListReview will emit the create event for an access list review.
func (s *Service) emitCreateAccessListReview(ctx context.Context, username string, review *accesslist.Review, createErr error) {
	code := events.AccessListReviewSuccessCode
	var errorMsg string
	if createErr != nil {
		errorMsg = createErr.Error()
		code = events.AccessListReviewFailureCode
	}

	var membershipRequirementsChanged *apievents.AccessListReviewMembershipRequirementsChanged
	if review.Spec.Changes.MembershipRequirementsChanged != nil {
		membershipRequirementsChanged = &apievents.AccessListReviewMembershipRequirementsChanged{
			Roles: review.Spec.Changes.MembershipRequirementsChanged.Roles,
		}
		if len(review.Spec.Changes.MembershipRequirementsChanged.Traits) > 0 {
			membershipRequirementsChanged.Traits = map[string]string{}

			// It was not intentional to have the event use a map[string]string for traits, but given that this is
			// purely for display I think this is okay.
			for trait, values := range review.Spec.Changes.MembershipRequirementsChanged.Traits {
				membershipRequirementsChanged.Traits[trait] = strings.Join(values, ",")
			}
		}
	}

	event := &apievents.AccessListReview{
		Metadata: apievents.Metadata{
			Type: events.AccessListReviewEvent,
			Code: code,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      review.Spec.AccessList,
			UpdatedBy: username,
		},
		AccessListReviewMetadata: apievents.AccessListReviewMetadata{
			Message:                       review.Spec.Notes,
			ReviewID:                      review.GetName(),
			MembershipRequirementsChanged: membershipRequirementsChanged,
			ReviewFrequencyChanged:        review.Spec.Changes.ReviewFrequencyChanged.String(),
			ReviewDayOfMonthChanged:       review.Spec.Changes.ReviewDayOfMonthChanged.String(),
			RemovedMembers:                review.Spec.Changes.RemovedMembers,
		},
		Status: apievents.Status{
			Success: createErr == nil,
			Error:   errorMsg,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.log.WithError(emitErr).Warnf("Failed to emit access list review create: %v", event)
	}
}

// DeleteAccessListReview will delete an access list review from the backend.
func (s *Service) DeleteAccessListReview(ctx context.Context, req *accesslistv1.DeleteAccessListReviewRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}

	_, err = authz.AuthorizeContextWithVerbs(ctx, s.log, authCtx, true, ruleCtx, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.accessListReviews.DeleteAccessListReview(ctx, req.AccessListName, req.ReviewName); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// Check if the user is either authorized for the access list or owns this access list.
// Returns early if user has RBAC access (skips the step for retrieving an access list).
func (s *Service) authOrIsOwner(ctx context.Context, accessListName string, verbs ...string) error {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	// Exit early if user has RBAC access to access lists.
	if hasAccess := s.hasAccessListRBAC(ctx, authCtx, verbs...); hasAccess {
		return nil
	}

	// Otherwise, we need to check if the user owns the access list.
	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		s.log.WithError(err).Debug("Failed to get access list")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	if err := s.isOwnerOfAccessList(ctx, authCtx, accessList); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// authOrIsOwnerWithAccessList first checks if retrieving access list was successful,
// then checks if the user is either authorized for the access list or owns this access list.
func (s *Service) authOrIsOwnerWithAccessList(ctx context.Context, accessListName string, verbs ...string) (*accesslist.AccessList, error) {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		s.log.WithError(err).Debug("Failed to get access list")
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	if hasAccess := s.hasAccessListRBAC(ctx, authCtx, verbs...); hasAccess {
		return accessList, nil
	}
	if err := s.isOwnerOfAccessList(ctx, authCtx, accessList); err != nil {
		return nil, trace.Wrap(err)
	}

	return accessList, nil
}

type StillEligibleFields struct {
	userLookup map[string]types.User
	requires   accesslist.Requires
	username   string
	expires    time.Time
	clock      clockwork.Clock
}

func checkUserIsStillEligible(f StillEligibleFields) accesslistv1.IneligibleStatus {
	foundUser, exists := f.userLookup[f.username]
	// Check if owner exists.
	if !exists {
		// We are skipping checks for user not found because a user
		// may be an SSO user and may not exist in the backend yet
		// from not having logged in for the day.
		//
		// SSO users are recorded dynamically. They get recorded upon login
		// and set to be deleted to the length of their session.
		//
		// In the case of large SSO users being added to an access list,
		// a lot of the users may appear to "not exist" and confuse
		// the viewer.
		return accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED
	}

	// Check if expired.
	if !f.expires.IsZero() && !f.clock.Now().Before(f.expires) {
		return accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED
	}

	// Check if user still meets requirements.
	ownerMeetsRequirements := services.UserMeetsRequirements(tlsca.Identity{
		Groups: foundUser.GetRoles(),
		Traits: foundUser.GetTraits(),
	}, f.requires)
	if !ownerMeetsRequirements {
		return accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS
	}

	return accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE
}

func makeUserLookup(users []types.User) map[string]types.User {
	userLookup := map[string]types.User{}
	for _, user := range users {
		userLookup[user.GetName()] = user
	}
	return userLookup
}

// memberEventMetadata is a small wrapper around a member object.
type memberEventMetadata struct {
	name     string
	reason   string
	joinedOn time.Time
}

// accessListMemberProtoToMemberEventMetadata converts a member proto into a memberNameAndReason.
func accessListMemberProtoToMemberEventMetadata(member *accesslistv1.Member) *memberEventMetadata {
	if member == nil || member.Spec == nil {
		return nil
	}

	return &memberEventMetadata{
		name:     member.Spec.Name,
		reason:   member.Spec.Reason,
		joinedOn: member.Spec.Joined.AsTime(),
	}
}

// accessListMembesrToMemberEventMetadata converts all members into a memberNameAndReason.
func accessListMembersToMemberEventMetadata(members []*accesslist.AccessListMember) []*memberEventMetadata {
	convertedMembers := []*memberEventMetadata{}
	for _, member := range members {
		if member == nil {
			return nil
		}

		convertedMembers = append(convertedMembers, &memberEventMetadata{
			name:     member.Spec.Name,
			reason:   member.Spec.Reason,
			joinedOn: member.Spec.Joined,
		})
	}

	return convertedMembers
}

// accessListMembersForEvent takes a proto version of an access list member and converts it into member metadata to be
// emitted in an event. The joinTime will override the joinedOn field in the memberEventMetadata object.
func accessListMembersForEvent(joinTime, removeTime time.Time, members ...*memberEventMetadata) []*apievents.AccessListMember {
	eventMembers := make([]*apievents.AccessListMember, 0)

	for _, member := range members {
		if member == nil {
			continue
		}

		joinedOn := member.joinedOn
		if joinedOn.IsZero() {
			joinedOn = joinTime
		}

		eventMember := &apievents.AccessListMember{
			JoinedOn:   joinedOn,
			RemovedOn:  removeTime,
			Reason:     member.reason,
			MemberName: member.name,
		}

		eventMembers = append(eventMembers, eventMember)
	}

	return eventMembers
}

// batchAccessListMemberMetadata will create batches of access list member metadata objects for emitting events in batches.
func batchAccessListMemberMetadata(accessListName string, members []*apievents.AccessListMember) []apievents.AccessListMemberMetadata {
	numMembers := len(members)
	numBatches := int(math.Ceil(float64(numMembers) / float64(eventMemberBatches)))
	batches := make([]apievents.AccessListMemberMetadata, numBatches)

	for i := 0; i < numBatches; i++ {
		startIndex := i * eventMemberBatches
		endIndex := startIndex + eventMemberBatches
		if endIndex > numMembers {
			endIndex = numMembers
		}
		batches[i] = apievents.AccessListMemberMetadata{
			AccessListName: accessListName,
			Members:        members[startIndex:endIndex],
		}
	}

	return batches
}
