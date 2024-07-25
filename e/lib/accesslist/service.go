package accesslist

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
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
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	// defaultAccessListPageSize is the default page size to be used.
	defaultAccessListPageSize = 100

	// eventMemberBatches is the number of members to emit per event. This will batch member events emitted by this service.
	eventMemberBatches = 50

	// oktaErrorMsg is the message to display when the modification of an Okta access list is attempted.
	oktaErrorMsg = "Okta sourced access lists cannot be modified"

	componentAccessListService = "access_list_crud_service"
)

type AuthServer interface {
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	SubmitAccessReview(ctx context.Context, req types.AccessReviewSubmission) (types.AccessRequest, error)
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)

	GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
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

	// UsageReporter is the reporter for sending usage without it be related to an API call.
	UsageReporter usagereporter.UsageReporter

	// Clock is the clock.
	Clock clockwork.Clock

	// Cache is the Auth server cache.
	Cache Cache

	// AuthServer implements the minimal auth server interface.
	AuthServer AuthServer

	// Backend is the backend to use.
	Backend backend.Backend
	// disableReconciler is a flag to disable the reconciler
	// for access list ineligibility updates during tests.
	disableReconciler bool
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

	if c.Cache == nil {
		return trace.BadParameter("Cache is missing")
	}

	if c.AuthServer == nil {
		return trace.BadParameter("auth server is missing")
	}

	if c.Backend == nil {
		return trace.BadParameter("backend is missing")
	}

	if c.Logger == nil {
		c.Logger = logrus.New().WithField(teleport.ComponentKey, componentAccessListService)
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
	usageReporter     usagereporter.UsageReporter
	emitter           apievents.Emitter
	clock             clockwork.Clock
	cache             Cache
	authServer        AuthServer
	backend           backend.Backend

	// When not set, this will use the default page size for ListUsers.
	userPageSize int
}

// NewService creates a new Access List gRPC service.
func NewService(ctx context.Context, cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		log:         cfg.Logger,
		authorizer:  cfg.Authorizer,
		accessLists: cfg.AccessLists,
		membershipChecker: services.NewAccessListMembershipChecker(
			cfg.Clock, cfg.AccessLists, cfg.LockGetter),
		accessListReviews: cfg.AccessListReviews,
		usageEvents:       cfg.UsageEvents,
		usageReporter:     cfg.UsageReporter,
		emitter:           cfg.Emitter,
		clock:             cfg.Clock,
		cache:             cfg.Cache,
		authServer:        cfg.AuthServer,
		backend:           cfg.Backend,
	}

	if !cfg.disableReconciler {
		go s.runAccessListIneligibleReconciler(ctx)
	}

	return s, nil
}

// GetAccessLists returns a list of all access lists.
func (s *Service) GetAccessLists(ctx context.Context, _ *accesslistv1.GetAccessListsRequest) (*accesslistv1.GetAccessListsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We don't return these errors right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	results, getErr := s.accessLists.GetAccessLists(ctx)

	authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList)

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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.PageSize)

	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}
	// We don't return the auth error right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList)

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

	if len(results) == 0 && authErr != nil {
		return nil, trace.Wrap(authErr)
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
// * If the user has RBAC access to the access lists (authErr == nil), the access lists will be returned with membership information added.
// * If the user owns any access lists, these will be returned with membership information added.
// * If the user is a member of any access lists, these will be returned without membership information.
func (s *Service) filterResults(ctx context.Context, results []*accesslist.AccessList, isPaginated bool, getErr, authErr error) ([]*accesslist.AccessList, error) {
	isMemberMap := map[string]bool{}
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

		for _, result := range results {
			isMember, err := s.userCanReadAccessList(ctx, authCtx, result, types.VerbRead, types.VerbList)
			isMemberMap[result.GetName()] = isMember
			if err == nil {
				filteredResults = append(filteredResults, result)
			}
		}

		results = filteredResults
	}

	// The user owns no access lists and received an auth err earlier. Also, we're not looking
	// at paginated lists.
	if len(results) == 0 && !isPaginated {
		return nil, trace.Wrap(authErr)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	// Add in member counts if appropriate.
	for _, result := range results {
		s.addMemberCounts(ctx, isMemberMap[result.GetName()], result)
	}

	return results, nil
}

// userCanReadAccessList will return no error if the user is an owner, a member, or has RBAC access to the access list.
// True will be returned if the user can only read the access list because they are a member.
func (s *Service) userCanReadAccessList(ctx context.Context, authCtx *authz.Context, accessList *accesslist.AccessList, verb string, additionalVerbs ...string) (bool, error) {
	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, verb, additionalVerbs...)

	// If access is explicitly denied, we'll not allow owner or membership checks.
	// We also can't do owner or membership checks if accessList is nil.
	if services.IsAccessExplicitlyDenied(authErr) || accessList == nil {
		return false, trace.Wrap(authErr)
	}

	// Allow the user to access the list if they are an owner or member.
	identity := authCtx.Identity.GetIdentity()
	if services.IsAccessListOwner(identity, accessList) == nil {
		return false, nil
	}

	if s.membershipChecker.IsAccessListMember(ctx, identity, accessList) == nil {
		return true, nil
	}

	return false, trace.Wrap(authErr)
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, getErr := s.accessLists.GetAccessList(ctx, req.GetName())

	// If we can get the access list, authorize using it.
	isMember, err := s.userCanReadAccessList(ctx, authCtx, result, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	s.addMemberCounts(ctx, isMember, result)

	// Get a list of all users, to compute eligibility for owners.
	users, err := getAllUsers(ctx, s.cache, s.userPageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	updatedOwners := applyOwnersIneligibleStatus(result, s.clock, userLookup)
	result.SetOwners(updatedOwners)

	return conv.ToProto(result), nil
}

// getAllUsers returns all users known to Teleport.
func getAllUsers(ctx context.Context, cache Cache, pageSize int) ([]types.User, error) {
	var users []types.User
	req := userspb.ListUsersRequest{
		PageSize: int32(pageSize),
	}
	for {
		rsp, err := cache.ListUsers(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, u := range rsp.Users {
			users = append(users, u)
		}

		req.PageToken = rsp.NextPageToken
		if req.PageToken == "" {
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
	rsp, err := s.updateOrUpsertAccessList(ctx, req.GetAccessList(), s.upsertAccessList)
	return rsp, trace.Wrap(err)
}

// UpdateAccessList updates an access list resource.
func (s *Service) UpdateAccessList(ctx context.Context, req *accesslistv1.UpdateAccessListRequest) (*accesslistv1.AccessList, error) {
	rsp, err := s.updateOrUpsertAccessList(ctx, req.GetAccessList(), s.updateAccessList)
	return rsp, trace.Wrap(err)
}

// updateOrUpsertAccessListSigFunc is a function that will upsert or update an access list.
// it's the signature for
type updateOrUpsertAccessListSigFunc func(ctx context.Context, authCtx *authz.Context, newAccessList *accesslist.AccessList) (resp *accesslistv1.AccessList, err error)

func (s *Service) updateOrUpsertAccessList(ctx context.Context, accessList *accesslistv1.AccessList, funcOpts updateOrUpsertAccessListSigFunc) (*accesslistv1.AccessList, error) {
	newAccessList, err := conv.FromProto(accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListName := newAccessList.GetName()

	oldAccessList, getErr := s.accessLists.GetAccessList(ctx, accessListName)
	if getErr != nil && !trace.IsNotFound(getErr) {
		return nil, trace.AccessDenied("access denied")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verb := types.VerbCreate
	if oldAccessList != nil {
		verb = types.VerbUpdate
		if err := authCtx.CheckAccessToResource(oldAccessList, verb); err != nil {
			return nil, trace.Wrap(err)
		}

		// Update the revision to make sure that the future upsert is rejected if somebody else has modified it while we're
		// running this function.
		newAccessList.SetRevision(oldAccessList.GetRevision())
	}

	if err := authCtx.CheckAccessToResource(newAccessList, verb); err != nil {
		return nil, trace.Wrap(err)
	}

	if !oktaModificationAllowed(*authCtx, oldAccessList, newAccessList) {
		return nil, trace.AccessDenied(oktaErrorMsg)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated := oldAccessList != nil
	resp, upsertErr := funcOpts(ctx, authCtx, newAccessList)

	s.emitUpsertAccessListEvent(ctx, username, updated, accessListName, upsertErr)

	if upsertErr == nil {
		s.emitUpsertAccessListUsageEvent(ctx, updated, accessListName)
	}

	return resp, trace.Wrap(upsertErr)
}

// upsertAccessList is a helper for upserting the access list that returns the response, whether this was an update request, and an error.
func (s *Service) upsertAccessList(ctx context.Context, authCtx *authz.Context, newAccessList *accesslist.AccessList) (resp *accesslistv1.AccessList, err error) {
	responseAccessList, err := s.accessLists.UpsertAccessList(ctx, newAccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(responseAccessList), nil
}

// updateAccessList is a helper for updating the access list that returns the response, whether this was an update request, and an error.
func (s *Service) updateAccessList(ctx context.Context, authCtx *authz.Context, newAccessList *accesslist.AccessList) (resp *accesslistv1.AccessList, err error) {
	responseAccessList, err := s.accessLists.UpdateAccessList(ctx, newAccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(responseAccessList), nil
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
	// ignore errors, we just want the access list if it exists for rbac purposes.
	accessList, _ := s.accessLists.GetAccessList(ctx, req.GetName())

	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, types.VerbDelete)
	if authErr != nil {
		return nil, trace.Wrap(authErr)
	}

	// Allow reused MFA responses to allow deleting an access list after deleting all members.
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.accessLists.DeleteAccessList(ctx, req.GetName()); err != nil {
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

// CountAccessListMembers will count all access list members.
func (s *Service) CountAccessListMembers(ctx context.Context, req *accesslistv1.CountAccessListMembersRequest) (*accesslistv1.CountAccessListMembersResponse, error) {
	_, err := s.authOrIsOwner(ctx, req.AccessListName, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	count, err := s.accessLists.CountAccessListMembers(ctx, req.AccessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &accesslistv1.CountAccessListMembersResponse{
		Count: count,
	}, nil
}

// ListAccessListMembers returns a paginated list of all access list members.
func (s *Service) ListAccessListMembers(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	retrievedAccessList, _, err := s.authOrIsOwnerWithAccessList(ctx, req.AccessList, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := s.accessLists.ListAccessListMembers(ctx, req.AccessList, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a list of all users, to compute eligibility for members.
	users, err := getAllUsers(ctx, s.cache, s.userPageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	members := applyMembersIneligibleStatus(results, retrievedAccessList.GetMembershipRequires(), s.clock, userLookup)

	return &accesslistv1.ListAccessListMembersResponse{
		Members:       members,
		NextPageToken: nextToken,
	}, nil
}

// GetAccessListMember returns the specified access list member resource.
func (s *Service) GetAccessListMember(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslistv1.Member, error) {
	if _, err := s.authOrIsOwner(ctx, req.AccessList, types.VerbRead); err != nil {
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
	authCtx, err := s.authOrIsOwner(ctx, req.Member.Spec.AccessList, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.Member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, accessListName, updated, upsertErr := s.upsertAccessListMember(ctx, authCtx, member, s.accessLists.UpsertAccessListMember)

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

// UpdateAccessListMember updates an access list member resource.
func (s *Service) UpdateAccessListMember(ctx context.Context, req *accesslistv1.UpdateAccessListMemberRequest) (*accesslistv1.Member, error) {
	authCtx, err := s.authOrIsOwner(ctx, req.Member.Spec.AccessList, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.Member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, accessListName, updated, upsertErr := s.upsertAccessListMember(ctx, authCtx, member, s.accessLists.UpdateAccessListMember)

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

// updateOrUpsertSignature is a function signature for updating or upserting access list members.
type updateOrUpsertSignature func(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)

// upsertAccessListMember is a helper for creating or updating access list members that returns the response, whether this was an update, and an error.
func (s *Service) upsertAccessListMember(ctx context.Context, authCtx *authz.Context, member *accesslist.AccessListMember, f updateOrUpsertSignature) (resultProto *accesslistv1.Member, accessListName string, updated bool, err error) {
	updated = false
	username, err := getUsername(authCtx)
	if err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	// If the user didn't exist before, make sure the current user is recorded as the user that added it.
	if oldMember, err := s.accessLists.GetAccessListMember(ctx, member.Spec.AccessList, member.GetName()); err == nil || trace.IsNotFound(err) {
		updated, member = populateMemberFields(s.clock, username, oldMember, member)
	} else {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	if err := s.userTryingToAddThemselves(ctx, authCtx, username, member.GetName(), member.Spec.Name); err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	result, err := f(ctx, member)
	if err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), member.Spec.AccessList, updated, nil
}

// userTryingToAddThemselves returns an error if the provided member matches the current username and that user.
func (s *Service) userTryingToAddThemselves(ctx context.Context, authCtx *authz.Context, username string, memberNames ...string) error {
	// If the member names contains the given username and the user doesn't have create/update access
	// to users, the user can't add themselves. If the user has create/update access to users, then
	// the user is able to add themselves.
	if slices.Contains(memberNames, username) && !s.hasUserRBAC(ctx, authCtx, types.VerbCreate, types.VerbUpdate) {
		// if slices.Contains(memberNames, username) {
		return trace.AccessDenied("user cannot add themselves to an access list")
	}

	return nil
}

// populateMemberFields will populate member fields with their pre-existing values or calculate
// new values. True will be returned if the existing values were preserved.
func populateMemberFields(clock clockwork.Clock, username string, oldMember, member *accesslist.AccessListMember) (preserved bool, populatedMember *accesslist.AccessListMember) {
	if oldMember == nil {
		// Make sure that the member metadata name matches the name in the spec.
		member.Spec.Name = member.GetName()
		member.Spec.AddedBy = username
		member.Spec.Joined = clock.Now()
		return false, member
	}

	// If the user already existed, use existing values.
	member.Metadata = oldMember.Metadata
	member.Spec.AccessList = oldMember.Spec.AccessList
	member.Spec.Name = oldMember.Spec.Name
	member.Spec.Joined = oldMember.Spec.Joined
	member.Spec.AddedBy = oldMember.Spec.AddedBy
	member.Spec.Reason = oldMember.Spec.Reason

	// If the IneligibleStatus is empty, use the existing value.
	// Ineligibility is dynamic field calculated by the backend.
	// Where Backend sets this field to empty string when returning data to a client
	// and expecting that a client will not set this field.
	if member.Spec.IneligibleStatus == "" {
		member.Spec.IneligibleStatus = oldMember.Spec.IneligibleStatus
	}

	return true, member
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
	authCtx, err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
	authCtx, err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Allow reused MFA responses to allow deleting an access list after deleting all members.
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, updated, accessListModified, modifiedMembers, upsertErr := s.upsertAccessListWithMembers(ctx, authCtx, req)

	var accessListName string
	if resp != nil {
		accessListName = resp.AccessList.GetHeader().Metadata.Name
	}
	if accessListModified {
		s.emitUpsertAccessListEvent(ctx, username, updated, accessListName, upsertErr)

		if upsertErr == nil {
			s.emitUpsertAccessListUsageEvent(ctx, updated, accessListName)
		}
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

// upsertAccessListWithMembers is a helper for upserting an access list with members that returns the response,
// whether the access list was updated, the modified members, and an error.
func (s *Service) upsertAccessListWithMembers(ctx context.Context, authCtx *authz.Context,
	req *accesslistv1.UpsertAccessListWithMembersRequest) (resp *accesslistv1.UpsertAccessListWithMembersResponse, updated,
	accessListModified bool, modified *modifiedMembers, err error,
) {
	newAccessList, err := conv.FromProto(req.AccessList)
	if err != nil {
		return nil, false, false, nil, trace.Wrap(err)
	}

	oldAccessList, err := s.accessLists.GetAccessList(ctx, newAccessList.GetName())
	if oldAccessList != nil {
		updated = true

		// Update the revision to make sure that the future upsert is rejected if somebody else has modified it while we're
		// running this function.
		newAccessList.SetRevision(oldAccessList.GetRevision())
	}

	if err != nil && !trace.IsNotFound(err) {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	accessListModified = !accessListEqual(oldAccessList, newAccessList)

	// Modifying the access list requires RBAC access.
	var authErrOld error

	verb := types.VerbCreate
	// Make sure the user has access to the old access list if it exists.
	if oldAccessList != nil {
		verb = types.VerbUpdate
		authErrOld = s.hasAccessListRBAC(ctx, authCtx, oldAccessList, verb)
		if services.IsAccessExplicitlyDenied(authErrOld) {
			return nil, updated, accessListModified, nil, trace.Wrap(authErrOld)
		}
	}

	// Make sure the user also has access to the access list to be created.
	authErrNew := s.hasAccessListRBAC(ctx, authCtx, newAccessList, verb)
	if services.IsAccessExplicitlyDenied(authErrNew) {
		return nil, updated, accessListModified, nil, trace.Wrap(authErrNew)
	}

	hasRBAC := authErrOld == nil && authErrNew == nil
	isOwner := s.isOwnerOfAccessList(ctx, authCtx, newAccessList) == nil

	if accessListModified && !oktaModificationAllowed(*authCtx, oldAccessList, newAccessList) {
		return nil, updated, accessListModified, nil, trace.AccessDenied(oktaErrorMsg)
	}

	// The logic here is as follows:
	// - If the user has RBAC permissions, anything is permitted.
	// - Non-RBAC users cannot modify the access list.
	// - Owners can add or remove members from the list.
	if !hasRBAC {
		if !isOwner {
			return nil, updated, accessListModified, nil, trace.AccessDenied("user does not own this access list")
		}
		if accessListModified {
			return nil, updated, accessListModified, nil, trace.AccessDenied("user cannot modify the access list")
		}
	}

	// Get the old members here, also used for emitting events.
	oldMembers, err := s.getAccessListMemberMap(ctx, req.AccessList.Header.Metadata.GetName())
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	// Convert members
	members := make([]*accesslist.AccessListMember, 0, len(req.Members))
	for _, member := range req.Members {
		m, err := conv.FromMemberProto(member)
		if err != nil {
			return nil, updated, accessListModified, nil, trace.Wrap(err)
		}
		// Preserve the added by and joined fields for old members.
		oldMember := oldMembers[m.GetName()]
		_, m = populateMemberFields(s.clock, username, oldMember, m)
		if err := s.canUpdateMembership(ctx, authCtx, username, oldMember, m); err != nil {
			return nil, updated, accessListModified, nil, trace.Wrap(err)
		}
		members = append(members, m)
	}

	// Call the API.
	updatedAccessList, updatedMembers, err := s.accessLists.UpsertAccessListWithMembers(ctx, newAccessList, members)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	// Figure out the member modifications for event emitting.
	modified = getModifiedMembers(oldMembers, updatedMembers)

	// Get a list of all users, to compute eligibility's.
	users, err := getAllUsers(ctx, s.cache, s.userPageSize)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	updatedProtoMembers := applyMembersIneligibleStatus(updatedMembers, updatedAccessList.GetMembershipRequires(), s.clock, userLookup)
	updatedOwners := applyOwnersIneligibleStatus(updatedAccessList, s.clock, userLookup)
	updatedAccessList.SetOwners(updatedOwners)

	// Return the updated access list and members.
	return &accesslistv1.UpsertAccessListWithMembersResponse{
		AccessList: conv.ToProto(updatedAccessList),
		Members:    updatedProtoMembers,
	}, updated, accessListModified, modified, nil
}

// canUpdateMembership will return an error if the given user is able to update membership for this member.
func (s *Service) canUpdateMembership(ctx context.Context, authCtx *authz.Context, username string,
	oldMember, newMember *accesslist.AccessListMember,
) error {
	err := s.userTryingToAddThemselves(ctx, authCtx, username, newMember.GetName(), newMember.Spec.Name)
	if err == nil {
		return nil
	}

	// This is a new member and the user is trying to add themselves.
	if oldMember == nil {
		return trace.Wrap(err)
	}

	// We want to make sure that if the user is touching their own membership within
	// an access list, they're not modifying the entry. The situation this covers
	// is roughly:
	// - owner1 adds owner2 to the access list.
	// - owner2 tries to add members to the access list, which includes owner2.
	// Without this special case, owner2 will never be able to add members to the list
	// since they're represented in it. With this check, so long as owner2 doesn't
	// actually modify their own entry, we can say that it's okay for them to
	// modify the users in this list.

	if !membersEqual(oldMember, newMember) {
		return trace.Wrap(err)
	}

	return nil
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

// hasAccessListRBAC tests if the user has RBAC access to the given access list,
// or access lists in general if no access list is given.
func (s *Service) hasAccessListRBAC(ctx context.Context, authCtx *authz.Context, accessList *accesslist.AccessList, verb string, additionalVerbs ...string) error {
	var authErr error
	if accessList != nil {
		authErr = authCtx.CheckAccessToResource(accessList, verb, additionalVerbs...)
	} else {
		authErr = authCtx.CheckAccessToKind(types.KindAccessList, verb, additionalVerbs...)
	}

	if authErr != nil && !trace.IsAccessDenied(authErr) {
		s.log.WithError(authErr).Debug("hasAccessListRBAC had unexpected error")
	}

	return trace.Wrap(authErr)
}

// hasUserRBAC tests if the user has RBAC access to users.
func (s *Service) hasUserRBAC(ctx context.Context, authCtx *authz.Context, verb string, additionalVerbs ...string) bool {
	authErr := authCtx.CheckAccessToKind(types.KindUser, verb, additionalVerbs...)
	if authErr != nil {
		s.log.WithError(authErr).Debug("hasUserRBAC had error")
	}

	return authErr == nil
}

// oktaModificationAllowed will return true if an Okta modification is allowed. If the access list is not an Okta object,
// this will return true.
func oktaModificationAllowed(authCtx authz.Context, oldAccessList, newAccessList *accesslist.AccessList) bool {

	hasOktaOrigin := false
	// If *either* of the supplied access lists are marked as okta origin, the
	// special Okta rules start applying
	for _, accessList := range []*accesslist.AccessList{oldAccessList, newAccessList} {
		if accessList != nil && accessList.Origin() == types.OriginOkta {
			hasOktaOrigin = true
			break
		}
	}

	if !hasOktaOrigin {
		return true
	}

	if authz.HasBuiltinRole(authCtx, string(types.RoleOkta)) {
		return true
	}

	return isOktaAccessListModificationAllowed(oldAccessList, newAccessList)
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
	accessList, authCtx, err := s.authOrIsOwnerWithAccessList(ctx, req.AccessListName, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
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

	_, _, _, err = s.upsertAccessListMember(ctx, authCtx, &accesslist.AccessListMember{
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
	}, s.accessLists.UpsertAccessListMember)
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
	if _, err := s.authOrIsOwner(ctx, req.AccessList, types.VerbList, types.VerbRead); err != nil {
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

	authCtx, err := s.authOrIsOwner(ctx, review.Spec.AccessList, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := s.accessLists.GetAccessList(ctx, req.Review.Spec.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, updatedReview, createErr := s.createAccessListReview(ctx, review, authCtx, accessList, username)

	s.emitCreateAccessListReview(ctx, username, updatedReview, createErr)

	if createErr == nil {
		s.emitCreateAccessListReviewUsageEvent(ctx, accessList.GetName(), updatedReview, accessList.Spec.Audit.NextAuditDate)
	}

	return resp, trace.Wrap(createErr)
}

// createAccessListReview is a helper for creating the access list review that returns the response and an error.
// The updated access review will be returned on success, else the existing access review will be returned.
func (s *Service) createAccessListReview(ctx context.Context, review *accesslist.Review,
	authCtx *authz.Context, accessList *accesslist.AccessList, username string,
) (*accesslistv1.CreateAccessListReviewResponse, *accesslist.Review, error) {
	// We don't have to check if the error of hasAccessListRBAC is explicitly denied here because
	// authOrIsOwner would have caught it above.
	hasRBAC := s.hasAccessListRBAC(ctx, authCtx, accessList, types.VerbCreate, types.VerbUpdate) == nil
	accessListModified := !isReviewChangesAllowed(review.Spec.Changes)

	// Make sure the owner can't modify the access list.
	// We don't need to do any Okta specific checks here, as the things that users can modify during a review are
	// all things acceptable to Okta.
	if accessListModified && !hasRBAC {
		return nil, review, trace.AccessDenied("user cannot modify the access list as part of the review")
	}

	// Make sure the reviewers reflect the current user and the review date is recorded as now.
	review.Spec.Reviewers = []string{username}
	review.Spec.ReviewDate = s.clock.Now()

	updatedReview, nextAuditDate, err := s.accessListReviews.CreateAccessListReview(ctx, review)
	if err != nil {
		return nil, review, trace.Wrap(err)
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

func (s *Service) emitCreateAccessListReviewUsageEvent(ctx context.Context, accessListName string, review *accesslist.Review, originalNextAuditDate time.Time) {
	if s.usageEvents == nil {
		return
	}

	daysSinceOriginalNextAuditDate := s.clock.Since(originalNextAuditDate) / (24 * time.Hour)

	event := &usageeventsv1.UsageEventOneOf{
		Event: &usageeventsv1.UsageEventOneOf_AccessListReviewCreate{
			AccessListReviewCreate: &usageeventsv1.AccessListReviewCreate{
				Metadata: &usageeventsv1.AccessListMetadata{
					Id: accessListName,
				},
				DaysPastNextAuditDate:         int32(daysSinceOriginalNextAuditDate),
				MembershipRequirementsChanged: review.Spec.Changes.MembershipRequirementsChanged != nil,
				ReviewFrequencyChanged:        review.Spec.Changes.ReviewFrequencyChanged.String() != "",
				ReviewDayOfMonthChanged:       review.Spec.Changes.ReviewDayOfMonthChanged.String() != "",
				NumberOfRemovedMembers:        int32(len(review.Spec.Changes.RemovedMembers)),
			},
		},
	}

	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{Event: event}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list review create usage event")
	}
}

// DeleteAccessListReview will delete an access list review from the backend.
func (s *Service) DeleteAccessListReview(ctx context.Context, req *accesslistv1.DeleteAccessListReviewRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ignore errors, we just want the access list if it exists for rbac purposes.
	accessList, _ := s.accessLists.GetAccessList(ctx, req.AccessListName)

	if err := s.hasAccessListRBAC(ctx, authCtx, accessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.accessListReviews.DeleteAccessListReview(ctx, req.AccessListName, req.ReviewName); err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitDeleteAccessListReviewUsageEvent(ctx, req.AccessListName, req.ReviewName)

	return &emptypb.Empty{}, nil
}

func (s *Service) emitDeleteAccessListReviewUsageEvent(ctx context.Context, accessListName, reviewID string) {
	if s.usageEvents == nil {
		return
	}

	event := &usageeventsv1.UsageEventOneOf{
		Event: &usageeventsv1.UsageEventOneOf_AccessListReviewDelete{
			AccessListReviewDelete: &usageeventsv1.AccessListReviewDelete{
				Metadata: &usageeventsv1.AccessListMetadata{
					Id: accessListName,
				},
				AccessListReviewId: reviewID,
			},
		},
	}

	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{Event: event}); err != nil {
		s.log.WithError(err).Warn("Failed to emit access list review delete usage event")
	}
}

// GetSuggestedAccessLists returns suggested access lists for an access request.
func (s *Service) GetSuggestedAccessLists(ctx context.Context, request *accesslistv1.GetSuggestedAccessListsRequest) (*accesslistv1.GetSuggestedAccessListsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity := authCtx.Identity.GetIdentity()
	suggestions, err := modules.GetModules().GetSuggestedAccessLists(ctx, &identity, s.authServer, s.accessLists, request.AccessRequestId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	suggestionsProto := make([]*accesslistv1.AccessList, len(suggestions))
	for i, r := range suggestions {
		suggestionsProto[i] = conv.ToProto(r)
	}

	return &accesslistv1.GetSuggestedAccessListsResponse{
		AccessLists: suggestionsProto,
	}, nil
}

// Check if the user is either authorized for the access list or owns this access list.
// Returns early if user has RBAC access (skips the step for retrieving an access list).
func (s *Service) authOrIsOwner(ctx context.Context, accessListName string, verb string, additionalVerbs ...string) (*authz.Context, error) {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	// Otherwise, we need to check if the user owns the access list.
	accessList, getErr := s.accessLists.GetAccessList(ctx, accessListName)

	// Exit early if user has RBAC access to access lists.
	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, verb, additionalVerbs...)
	if authErr == nil {
		return authCtx, nil
	} else if services.IsAccessExplicitlyDenied(authErr) {
		return nil, trace.Wrap(authErr)
	}

	if getErr != nil {
		s.log.WithError(err).Debug("Failed to get access list")
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	if err := s.isOwnerOfAccessList(ctx, authCtx, accessList); err != nil {
		return nil, trace.Wrap(err)
	}

	return authCtx, nil
}

// authOrIsOwnerWithAccessList first checks if retrieving access list was successful,
// then checks if the user is either authorized for the access list or owns this access list.
func (s *Service) authOrIsOwnerWithAccessList(ctx context.Context, accessListName string, verb string, additionalVerbs ...string) (*accesslist.AccessList, *authz.Context, error) {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return nil, nil, trace.AccessDenied("access denied")
	}

	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		s.log.WithError(err).Debug("Failed to get access list")
		// Return an opaque error
		return nil, nil, trace.AccessDenied("access denied")
	}

	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, verb, additionalVerbs...)
	if authErr == nil {
		return accessList, authCtx, nil
	} else if services.IsAccessExplicitlyDenied(authErr) {
		return nil, nil, trace.Wrap(authErr)
	} else if err := s.isOwnerOfAccessList(ctx, authCtx, accessList); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return accessList, authCtx, nil
}

// addMemberCounts to the given access list.
func (s *Service) addMemberCounts(ctx context.Context, isMember bool, accessList *accesslist.AccessList) {
	if isMember {
		return
	}

	memberCount, err := s.accessLists.CountAccessListMembers(ctx, accessList.GetName())
	if err != nil {
		s.log.WithError(err).Error("Error counting access list members")
		return
	}
	accessList.Status.MemberCount = &memberCount
}

func (s *Service) runAccessListIneligibleReconciler(ctx context.Context) error {
	const accessListIneligibleReconciler = "access_list_ineligible_reconciler"
	for {
		err := backend.RunWhileLocked(
			ctx,
			backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					LockName:      accessListIneligibleReconciler,
					Backend:       s.backend,
					TTL:           60 * time.Second,
					RetryInterval: 30 * time.Second,
				},
				ReleaseCtxTimeout:   60 * time.Second,
				RefreshLockInterval: 30 * time.Second,
			},
			func(ctx context.Context) error {
				reconciler, err := NewIneligibleStatusReconciler(
					ctx,
					IneligibleStatusReconcilerConfig{
						Cache:   s.cache,
						Service: s.accessLists,
						Log:     s.log.WithField("reconciler", accessListIneligibleReconciler),
						Clock:   s.clock,
					},
				)
				if err != nil {
					return trace.Wrap(err)
				}
				defer reconciler.Close()
				if err := reconciler.Run(ctx); err != nil {
					s.log.WithError(err).Error("Error running access list ineligible reconciler")
					return trace.Wrap(err)
				}
				return nil
			},
		)
		if err != nil {
			select {
			case <-s.clock.After(30 * time.Second):
			case <-ctx.Done():
				return trace.Wrap(err)
			}
			s.log.WithError(err).Error("Error running access list ineligible reconciler")
		}
	}
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

func getUsername(authCtx *authz.Context) (string, error) {
	if authCtx == nil {
		return "", trace.BadParameter("authCtx is nil")
	}

	if authz.HasBuiltinRole(*authCtx, string(types.RoleOkta)) {
		return "okta-service", nil
	}

	identity := authCtx.Identity.GetIdentity()

	return identity.Username, nil
}

// applyMembersIneligibleStatus goes through each member and determines eligibility.
// Returns a new list of proto converted members with applied status.
func applyMembersIneligibleStatus(members []*accesslist.AccessListMember, memberRequires accesslist.Requires, clock clockwork.Clock, userLookup map[string]types.User) []*accesslistv1.Member {
	updatedProtoMembers := make([]*accesslistv1.Member, len(members))
	for i, r := range members {
		ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
			userLookup: userLookup,
			username:   r.GetName(),
			expires:    r.Spec.Expires,
			clock:      clock,
			requires:   memberRequires,
		})
		r.Spec.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
		updatedProtoMembers[i] = conv.ToMemberProto(r)
	}
	return updatedProtoMembers
}

// applyOwnersIneligibleStatus goes through each owner and determines eligibility.
// Returns a new list of owners with applied status.
func applyOwnersIneligibleStatus(accessList *accesslist.AccessList, clock clockwork.Clock, userLookup map[string]types.User) []accesslist.Owner {
	updatedOwners := make([]accesslist.Owner, len(accessList.GetOwners()))
	for i, owner := range accessList.GetOwners() {
		ineligibleStatus := checkUserIsStillEligible(StillEligibleFields{
			userLookup: userLookup,
			username:   owner.Name,
			expires:    time.Time{}, // owners don't have expiry's
			clock:      clock,
			requires:   accessList.GetOwnershipRequires(),
		})

		owner.IneligibleStatus = accesslistv1.IneligibleStatus_name[int32(ineligibleStatus)]
		updatedOwners[i] = owner
	}

	return updatedOwners
}
