package accesslist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/accesslist/preset"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	// defaultAccessListPageSize is the default page size to be used.
	defaultAccessListPageSize = 100

	// eventMemberBatches is the number of members to emit per event. This will batch member events emitted by this service.
	eventMemberBatches = 50

	componentAccessListService = "access_list_crud_service"

	// OktaServiceRoleUsername is the name of the user for the Teleport Okta Identity
	OktaServiceRoleUsername = "okta-service"
)

type AuthServer interface {
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
	GetAccessList(ctx context.Context, list string) (*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, list string, user string) (*accesslist.AccessListMember, error)

	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	SubmitAccessReview(ctx context.Context, req types.AccessReviewSubmission) (types.AccessRequest, error)
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)

	GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
	UpsertRole(ctx context.Context, r types.Role) (types.Role, error)
	UpdateRole(ctx context.Context, r types.Role) (types.Role, error)
	CreateRole(ctx context.Context, r types.Role) (types.Role, error)

	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger *slog.Logger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// AccessLists is the access list service to use.
	AccessLists services.AccessListsInternal

	// LockGetter is a getter for locks.
	LockGetter services.LockGetter

	// AccessListReviews is the access list reviews service to use.
	AccessListReviews services.AccessListReviews

	// Plugins is the plugins service to use.
	Plugins services.Plugins

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

	// Modules defines build time constraints and licensed features.
	Modules modules.Modules

	// disableReconcilers is a flag to disable ineligibility and status reconcilers to avoid
	// extra update events during the tests.
	disableReconcilers bool
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

	if c.Plugins == nil {
		c.Plugins = local.NewPluginsService(c.Backend)
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
	if c.Modules == nil {
		return trace.BadParameter("modules is missing")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, componentAccessListService)
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

type nonStaticAccessListError struct {
	accessList, member string
	accessListType     accesslist.Type
}

func newNonStaticAccessListErrorFromMemberReq(req memberGetter, accessListType accesslist.Type) *nonStaticAccessListError {
	return &nonStaticAccessListError{
		accessList:     req.GetMember().GetSpec().GetAccessList(),
		member:         req.GetMember().GetHeader().GetMetadata().GetName(),
		accessListType: accessListType,
	}
}

func newNonStaticAccessListErrorFromMemberMetaReq(req memberMetaGetter, accessListType accesslist.Type) *nonStaticAccessListError {
	return &nonStaticAccessListError{
		accessList:     req.GetAccessList(),
		member:         req.GetMemberName(),
		accessListType: accessListType,
	}
}

func (e *nonStaticAccessListError) Error() string {
	friendlyType := fmt.Sprintf("%q", string(e.accessListType))
	if e.accessListType == accesslist.Default {
		friendlyType += " (default)"
	}
	return fmt.Sprintf(
		`Access list member's (%[1]q) access list (%[2]q) is not static (i.e., access_list with spec.type set to "static"). Access list %[2]q type is %[3]s. Teleport IaC tools support adding members only to access lists of type "static".`,
		e.member, e.accessList, friendlyType,
	)
}

func (e *nonStaticAccessListError) Unwrap() error {
	return &trace.BadParameterError{
		Message: e.Error(),
	}
}

func isNonStaticAccessList(err error) bool {
	var val nonStaticAccessListError
	ptr := &val
	return errors.As(err, &ptr)
}

type Service struct {
	accesslistv1.UnimplementedAccessListServiceServer

	logger            *slog.Logger
	authorizer        authz.Authorizer
	accessLists       services.AccessListsInternal
	accessListReviews services.AccessListReviews
	plugins           services.Plugins
	usageEvents       UsageEventsClient
	usageReporter     usagereporter.UsageReporter
	emitter           apievents.Emitter
	clock             clockwork.Clock
	cache             Cache
	authServer        AuthServer
	backend           backend.Backend
	lockGetter        services.LockGetter
	modules           modules.Modules
}

// NewService creates a new Access List gRPC service.
func NewService(ctx context.Context, cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		logger:            cfg.Logger,
		authorizer:        cfg.Authorizer,
		accessLists:       cfg.AccessLists,
		accessListReviews: cfg.AccessListReviews,
		plugins:           cfg.Plugins,
		usageEvents:       cfg.UsageEvents,
		usageReporter:     cfg.UsageReporter,
		emitter:           cfg.Emitter,
		clock:             cfg.Clock,
		cache:             cfg.Cache,
		lockGetter:        cfg.LockGetter,
		authServer:        cfg.AuthServer,
		backend:           cfg.Backend,
		modules:           cfg.Modules,
	}

	if !cfg.disableReconcilers {
		go func() {
			if err := s.runAccessListIneligibleReconciler(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.ErrorContext(ctx, "Access List ineligible reconciler exited with error.", "error", err)
			}
		}()

		go func() {
			if err := s.runAccessListStatusReconciler(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.ErrorContext(ctx, "Access List status reconciler exited with error.", "error", err)
			}
		}()
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
	results, getErr := s.cache.GetAccessLists(ctx)

	authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList)

	// There was an error getting the access lists and an auth error, so return the auth error.
	// If we can't get the access lists, we can't check ownership/membership.
	if getErr != nil && authErr != nil {
		return nil, trace.Wrap(authErr)
	}

	results, err = s.filterResults(ctx, results, false, getErr, authErr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return accesslistv1.GetAccessListsResponse_builder{
		AccessLists: accessLists,
	}.Build(), nil
}

// ListAccessListsV2 returns a paginated list of all access lists.
func (s *Service) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) (*accesslistv1.ListAccessListsV2Response, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.GetPageSize())

	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}
	// We don't return the auth error right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList)

	var results []*accesslist.AccessList
	var nextToken string
	for {
		var page []*accesslist.AccessList
		var getErr error
		page, nextToken, getErr = s.cache.ListAccessListsV2(ctx, req)

		var err error
		page, err = s.filterResults(ctx, page, true, getErr, authErr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		results = append(results, page...)
		if len(results) >= (pageSize) || nextToken == "" {
			break
		}
		req.SetPageToken(nextToken)
	}

	if len(results) == 0 && authErr != nil {
		return nil, trace.Wrap(authErr)
	}
	sortBy := req.GetSortBy()
	indexName := "name"
	if sortBy != nil {
		indexName = sortBy.Field
	}

	// Truncate the results.
	if len(results) > pageSize {
		nextToken, err = services.CreateAccessListNextKey(results[pageSize], indexName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = results[:pageSize]
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return accesslistv1.ListAccessListsV2Response_builder{
		AccessLists:   accessLists,
		NextPageToken: nextToken,
	}.Build(), nil
}

// ListAccessLists returns a paginated list of all access lists.
// Deprecated: Use [ListAccessListsV2] instead.
func (s *Service) ListAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsRequest) (*accesslistv1.ListAccessListsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.GetPageSize())

	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}
	// We don't return the auth error right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList)

	var results []*accesslist.AccessList
	nextToken := req.GetNextToken()
	for {
		var page []*accesslist.AccessList
		var getErr error
		page, nextToken, getErr = s.cache.ListAccessLists(ctx, 0, nextToken)

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

	return accesslistv1.ListAccessListsResponse_builder{
		AccessLists: accessLists,
		NextToken:   nextToken,
	}.Build(), nil
}

// filterResults will return the following:
// * If the user has RBAC access to the access lists (authErr == nil), the access lists will be returned with membership information added.
// * If the user owns any access lists, these will be returned with membership information added.
// * If the user is a member of any access lists, these will be returned without membership information.
func (s *Service) filterResults(ctx context.Context, results []*accesslist.AccessList, isPaginated bool, getErr, authErr error) ([]*accesslist.AccessList, error) {
	isMemberMap := map[string]bool{}

	// There was an error getting the access lists and an auth error, so return the auth error.
	if getErr != nil && authErr != nil {
		return nil, trace.Wrap(authErr)
	}

	// We successfully got the access lists but had an issue authorizing. Check to see if the user is an
	// owner for any of these lists.
	if authErr != nil {
		var filteredResults []*accesslist.AccessList

		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, result := range results {
			currentAssignments, err := s.userCanReadAccessList(ctx, authCtx, result, types.VerbRead, types.VerbList)
			isMemberMap[result.GetName()] = currentAssignments.IsMember()
			result.Status.CurrentUserAssignments = &currentAssignments
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
func (s *Service) userCanReadAccessList(ctx context.Context, authCtx *authz.Context, accessList *accesslist.AccessList, verb string, additionalVerbs ...string) (accesslist.CurrentUserAssignments, error) {
	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, verb, additionalVerbs...)

	assignments := accesslist.CurrentUserAssignments{
		OwnershipType:  accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
		MembershipType: accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED,
	}

	// If access is explicitly denied, we'll not allow owner or membership checks.
	if services.IsAccessExplicitlyDenied(authErr) {
		return assignments, trace.Wrap(authErr)
	}

	// We also can't do owner or membership checks if accessList or authCtx are nil.
	if authCtx == nil {
		return assignments, trace.AccessDenied("access denied")
	}
	if accessList == nil {
		return assignments, trace.NotFound("Access List not found")
	}

	// Allow the user to access the list if they are an owner or member.
	if ownershipType, err := accesslists.IsAccessListOwner(ctx, authCtx.User, accessList, s.accessLists, s.lockGetter, s.clock); err == nil {
		assignments.OwnershipType = ownershipType
	}
	if membershipType, err := accesslists.IsAccessListMember(ctx, authCtx.User, accessList, s.accessLists, s.lockGetter, s.clock); err == nil {
		assignments.MembershipType = membershipType
	}

	if assignments.IsOwner() || assignments.IsMember() {
		return assignments, nil
	}

	return assignments, trace.Wrap(authErr)
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, getErr := s.cache.GetAccessList(ctx, req.GetName())

	// If we can get the access list, authorize using it.
	currentAssignments, err := s.userCanReadAccessList(ctx, authCtx, result, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	s.addMemberCounts(ctx, currentAssignments.IsMember(), result)

	// Get a list of all users, to compute eligibility for owners.
	users, err := getAllUsers(ctx, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	updatedOwners := applyOwnersIneligibleStatus(result, s.clock, userLookup)
	result.SetOwners(updatedOwners)

	result.Status.CurrentUserAssignments = &currentAssignments

	return conv.ToProto(result), nil
}

// getAllUsers returns all users known to Teleport.
func getAllUsers(ctx context.Context, cache Cache) ([]types.User, error) {
	iterFn := func(ctx context.Context, pageSize int, nextToken string) ([]*types.UserV2, string, error) {
		req := userspb.ListUsersRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: nextToken,
		}.Build()
		resp, err := cache.ListUsers(ctx, req)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.GetUsers(), resp.GetNextPageToken(), nil
	}

	var out []types.User
	for item, err := range clientutils.Resources(ctx, iterFn) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

// GetAccessListsToReview will return access lists that need to be reviewed by the current user.
func (s *Service) GetAccessListsToReview(ctx context.Context, req *accesslistv1.GetAccessListsToReviewRequest) (*accesslistv1.GetAccessListsToReviewResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &accesslistv1.GetAccessListsToReviewResponse{}
	now := s.clock.Now()
	accessLists, err := s.cache.GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, accessList := range accessLists {
		if !accessList.IsReviewable() {
			continue
		}
		if s.needsReviewBy(ctx, authCtx.User, accessList, now) {
			resp.SetAccessLists(append(resp.GetAccessLists(), conv.ToProto(accessList)))
		}
	}

	return resp, nil
}

// needsReviewBy returns true if the access list should be reviewed by the user.
func (s *Service) needsReviewBy(ctx context.Context, user types.User, accessList *accesslist.AccessList, now time.Time) bool {
	if accessList.Spec.Audit.NextAuditDate.Sub(now) > accessList.Spec.Audit.Notifications.Start {
		// Access List is not yet within the review window.
		// No need to check ownership.
		return false
	}

	// TODO(smallinsky) Switch to GetHierarchyForUser when it will be supported in v18.
	if ownershipType, err := accesslists.IsAccessListOwner(ctx, user, accessList, s.accessLists, s.lockGetter, s.clock); err == nil {
		if ownershipType != accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
			return true
		}
	}
	return false
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
		s.logger.WarnContext(ctx, "Failed to get Access List", "error", getErr)
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

	if err := s.checkModificationAllowed(*authCtx, oldAccessList, newAccessList); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated := oldAccessList != nil
	resp, upsertErr := funcOpts(ctx, authCtx, newAccessList)

	s.emitUpsertAccessListEvent(ctx, username, updated, accessListName, newAccessList.Spec.Title, upsertErr)

	if upsertErr == nil {
		s.emitUpsertAccessListUsageEvent(ctx, updated, accessListName)
	}

	return resp, trace.Wrap(upsertErr)
}

// GetInheritedGrants returns grants inherited by access list accessListID from parent access lists.
func (s *Service) GetInheritedGrants(ctx context.Context, req *accesslistv1.GetInheritedGrantsRequest) (*accesslistv1.GetInheritedGrantsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	acl, getErr := s.cache.GetAccessList(ctx, req.GetAccessListId())

	// If we can get the access list, authorize using it.
	_, err = s.userCanReadAccessList(ctx, authCtx, acl, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the getErr.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	grants, err := accesslists.GetInheritedGrants(ctx, acl, s.accessLists)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accesslistv1.GetInheritedGrantsResponse_builder{
		Grants: conv.ConvertGrantsToProto(*grants),
	}.Build(), nil
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
func (s *Service) emitUpsertAccessListEvent(ctx context.Context, username string, updated bool, accessListName string, accessListTitle string, upsertErr error) {
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
			AccessListTitle:  accessListTitle,
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
			AccessListTitle:  accessListTitle,
		}
		if upsertErr != nil {
			event.SetCode(events.AccessListCreateFailureCode)
		}
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list create/update event", "error", emitErr)
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
		s.logger.WarnContext(ctx, "Failed to emit access list create/update usage event", "error", err)
	}
}

// DeleteAccessList removes the specified access list resource.
func (s *Service) DeleteAccessList(ctx context.Context, req *accesslistv1.DeleteAccessListRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := s.accessLists.GetAccessList(ctx, req.GetName())
	if err != nil && !trace.IsNotFound(err) {
		s.logger.WarnContext(ctx, "Failed to get access list", "error", err)
	}
	resp, deleteErr := s.deleteAccessList(ctx, authCtx, req, accessList)
	var accessListTitle string
	if accessList != nil {
		accessListTitle = accessList.Spec.Title
	}
	s.emitDeleteAccessListEvent(ctx, authCtx, req.GetName(), accessListTitle, deleteErr)

	if deleteErr == nil {
		s.emitDeleteAccessListUsageEvent(ctx, req.GetName())
	}

	return resp, trace.Wrap(deleteErr)
}

// deleteAccessList is a helper for deleting the access list that returns the response and an error.
func (s *Service) deleteAccessList(ctx context.Context, authCtx *authz.Context, req *accesslistv1.DeleteAccessListRequest, accessList *accesslist.AccessList) (*emptypb.Empty, error) {
	authErr := s.hasAccessListRBAC(ctx, authCtx, accessList, types.VerbDelete)
	if authErr != nil {
		return nil, trace.Wrap(authErr)
	}

	if accessList == nil {
		return nil, trace.NotFound("Access List not found")
	}

	// Allow reused MFA responses to allow deleting an access list after deleting all members.
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkAccessListDeletionAllowed(ctx, *authCtx, accessList); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.accessLists.DeleteAccessList(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// emitDeleteAccessListEvent will emit the delete event for the access list.
func (s *Service) emitDeleteAccessListEvent(ctx context.Context, authCtx *authz.Context, accessListName string, accessListTitle string, deleteErr error) {
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
		AccessListTitle: accessListTitle,
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list delete event", "error", emitErr)
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
		s.logger.WarnContext(ctx, "Failed to emit access list delete usage event", "error", err)
	}
}

// CountAccessListMembers will count all access list members.
func (s *Service) CountAccessListMembers(ctx context.Context, req *accesslistv1.CountAccessListMembersRequest) (*accesslistv1.CountAccessListMembersResponse, error) {
	_, err := s.authOrIsOwner(ctx, req.GetAccessListName(), types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	count, listCount, err := s.cache.CountAccessListMembers(ctx, req.GetAccessListName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accesslistv1.CountAccessListMembersResponse_builder{
		Count:     count,
		ListCount: listCount,
	}.Build(), nil
}

// ListAccessListMembers returns a paginated list of all access list members.
func (s *Service) ListAccessListMembers(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	retrievedAccessList, _, err := s.authOrIsOwnerWithAccessList(ctx, req.GetAccessList(), types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := s.cache.ListAccessListMembers(ctx, req.GetAccessList(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a list of all users, to compute eligibility for members.
	users, err := getAllUsers(ctx, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	members := applyMembersIneligibleStatus(results, retrievedAccessList.GetMembershipRequires(), s.clock, userLookup)

	return accesslistv1.ListAccessListMembersResponse_builder{
		Members:       members,
		NextPageToken: nextToken,
	}.Build(), nil
}

// ListAllAccessListMembers returns a page of access lists members. The members returned
// span all existing access lists.
func (s *Service) ListAllAccessListMembers(ctx context.Context, req *accesslistv1.ListAllAccessListMembersRequest) (*accesslistv1.ListAllAccessListMembersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.logger.DebugContext(ctx, "Failed to authorize user", "error", err)
		// Return an opaque error
		return nil, trace.AccessDenied("access denied")
	}

	if authErr := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList); authErr != nil {
		if !trace.IsAccessDenied(authErr) {
			s.logger.DebugContext(ctx, "unexpected authorization error", "error", authErr)
		}
		return nil, trace.Wrap(authErr)
	}

	members, nextToken, err := s.cache.ListAllAccessListMembers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accesslistv1.ListAllAccessListMembersResponse_builder{
		NextPageToken: nextToken,
		Members:       conv.ToMembersProto(members),
	}.Build(), nil
}

// GetAccessListMember returns the specified access list member resource.
func (s *Service) GetAccessListMember(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslistv1.Member, error) {
	if err := validateMemberMetaRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := s.getAccessListMember(ctx, req, memberOptions{})
	return m, trace.Wrap(err)
}

// GetStaticAccessListMember returns the specified access_list_member resource. If returns error
// if the target access_list is not of type static.
func (s *Service) GetStaticAccessListMember(ctx context.Context, req *accesslistv1.GetStaticAccessListMemberRequest) (*accesslistv1.GetStaticAccessListMemberResponse, error) {
	if err := validateMemberMetaRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := s.getAccessListMember(ctx, req, memberOptions{
		requireStatic: true,
	})
	return accesslistv1.GetStaticAccessListMemberResponse_builder{Member: m}.Build(), trace.Wrap(err)
}

func (s *Service) getAccessListMember(ctx context.Context, req memberMetaGetter, opts memberOptions) (*accesslistv1.Member, error) {
	if _, err := s.authOrIsOwner(ctx, req.GetAccessList(), types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if opts.requireStatic {
		acl, err := s.accessLists.GetAccessList(ctx, req.GetAccessList())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		typ := acl.Spec.Type
		if typ != accesslist.Static {
			return nil, trace.Wrap(newNonStaticAccessListErrorFromMemberMetaReq(req, typ))
		}
	}

	result, err := s.cache.GetAccessListMember(ctx, req.GetAccessList(), req.GetMemberName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), nil
}

// UpsertAccessListMember creates or updates an access list member resource.
func (s *Service) UpsertAccessListMember(ctx context.Context, req *accesslistv1.UpsertAccessListMemberRequest) (*accesslistv1.Member, error) {
	opts := memberOptions{}
	if err := validateMemberRequest(req, opts); err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := s.upsertAccessListMember(ctx, req, opts)
	return m, trace.Wrap(err)
}

// UpsertStaticAccessListMember creates or updates an access_list_member resource. It returns error
// and does nothing if the target access_list is not of type static.
func (s *Service) UpsertStaticAccessListMember(ctx context.Context, req *accesslistv1.UpsertStaticAccessListMemberRequest) (*accesslistv1.UpsertStaticAccessListMemberResponse, error) {
	opts := memberOptions{
		requireStatic: true,
	}
	if err := validateMemberRequest(req, opts); err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := s.upsertAccessListMember(ctx, req, opts)
	return accesslistv1.UpsertStaticAccessListMemberResponse_builder{Member: m}.Build(), trace.Wrap(err)
}

func (s *Service) upsertAccessListMember(ctx context.Context, req memberGetter, opts memberOptions) (*accesslistv1.Member, error) {
	authCtx, err := s.authOrIsOwner(ctx, req.GetMember().GetSpec().GetAccessList(), types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.GetMember())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberAccessList, err := s.accessLists.GetAccessList(ctx, req.GetMember().GetSpec().GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if opts.requireStatic {
		typ := memberAccessList.Spec.Type
		if typ != accesslist.Static {
			return nil, trace.Wrap(newNonStaticAccessListErrorFromMemberReq(req, typ))
		}
	}

	if err := s.checkMembersModificationAllowed(ctx, *authCtx, memberAccessList); err != nil {
		return nil, trace.Wrap(err, "adding and updating members not allowed for Access List %q", req.GetMember().GetSpec().GetAccessList())
	}

	resp, accessListName, updated, upsertErr := s.runAccessListMemberOp(ctx, authCtx, member, s.accessLists.UpsertAccessListMember)

	var joinTime time.Time
	if resp != nil {
		joinTime = resp.GetSpec().GetJoined().AsTime()
	}

	s.emitUpsertAccessListMemberEvent(ctx, username, updated, accessListName, upsertErr,
		accessListMembersForEvent(joinTime, time.Time{}, accessListMemberProtoToMemberEventMetadata(req.GetMember()))...)

	if upsertErr == nil {
		s.emitUpsertAccessListMemberUsageEvent(ctx, updated, accessListName, member)
	}

	return resp, trace.Wrap(upsertErr)
}

// UpdateAccessListMember updates an access list member resource.
func (s *Service) UpdateAccessListMember(ctx context.Context, req *accesslistv1.UpdateAccessListMemberRequest) (*accesslistv1.Member, error) {
	authCtx, err := s.authOrIsOwner(ctx, req.GetMember().GetSpec().GetAccessList(), types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.GetMember())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if member.Kind == accesslist.MembershipKindList {
		return nil, trace.BadParameter("Access List Member entries of kind 'list' cannot be updated")
	}

	if err := s.checkMembersModificationAllowedByName(ctx, *authCtx, req.GetMember().GetSpec().GetAccessList()); err != nil {
		return nil, trace.Wrap(err, "updating members not allowed for Access List %q", req.GetMember().GetSpec().GetAccessList())
	}

	resp, accessListName, updated, upsertErr := s.runAccessListMemberOp(ctx, authCtx, member, s.accessLists.UpdateAccessListMember)

	var joinTime time.Time
	if resp != nil {
		joinTime = resp.GetSpec().GetJoined().AsTime()
	}

	s.emitUpsertAccessListMemberEvent(ctx, username, updated, accessListName, upsertErr,
		accessListMembersForEvent(joinTime, time.Time{}, accessListMemberProtoToMemberEventMetadata(req.GetMember()))...)

	if upsertErr == nil {
		s.emitUpsertAccessListMemberUsageEvent(ctx, updated, accessListName, member)
	}

	return resp, trace.Wrap(upsertErr)
}

// updateOrUpsertMemberSignature is a function signature for updating or upserting access list members.
type updateOrUpsertMemberSignature func(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)

// runAccessListMemberOp is a helper for creating or updating access list members that returns the response, whether this was an update, and an error.
func (s *Service) runAccessListMemberOp(ctx context.Context, authCtx *authz.Context, member *accesslist.AccessListMember, f updateOrUpsertMemberSignature) (resultProto *accesslistv1.Member, accessListName string, updated bool, err error) {
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

	// Validate user isn't trying to add or update themselves.
	if err := s.userTryingToAddThemselves(ctx, authCtx, username, member); err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	result, err := f(ctx, member)
	if err != nil {
		return nil, member.Spec.AccessList, updated, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), member.Spec.AccessList, updated, nil
}

// userTryingToAddThemselves returns an error if the provided member matches the current username and that user.
func (s *Service) userTryingToAddThemselves(ctx context.Context, authCtx *authz.Context, username string, members ...*accesslist.AccessListMember) error {
	// If the member names contains the given username and the user doesn't have create/update access
	// to users, the user can't add themselves. If the user has create/update access to users, then
	// the user is able to add themselves.
	if s.hasUserRBAC(ctx, authCtx, types.VerbCreate, types.VerbUpdate) {
		return nil
	}

	// If the user doesn't have create/update access, we want to ensure they're not directly adding themselves,
	// or trying to add an Access List they're an explicit or inherited member of.
	for _, member := range members {
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			// GetMembersFor doesn't validate the validity of the membership,
			// so this prevents a user with an expired membership to appear as
			// "not a member", add a list they are in, and get their membership
			// renewed.
			members, err := accesslists.GetMembersFor(ctx, member.GetName(), s.accessLists)
			if err != nil {
				return trace.Wrap(err)
			}
			for _, m := range members {
				if m.GetName() == username {
					return trace.AccessDenied("Adding an Access List you are a member of to another Access List is not allowed")
				}
			}
		} else if member.GetName() == username {
			return trace.AccessDenied("Adding yourself to an Access List is not allowed")
		}
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

	newLabelsClone := maps.Clone(member.Metadata.Labels)
	member.Metadata = oldMember.Metadata
	if len(newLabelsClone) > 0 {
		member.Metadata.Labels = newLabelsClone
	}

	// If the user already existed, use existing values.
	member.Spec.AccessList = oldMember.Spec.AccessList
	member.Spec.Name = oldMember.Spec.Name
	member.Spec.Joined = oldMember.Spec.Joined
	member.Spec.AddedBy = oldMember.Spec.AddedBy
	member.Spec.Reason = oldMember.Spec.Reason
	member.Spec.MembershipKind = oldMember.Spec.MembershipKind

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

	accessList, err := s.cache.GetAccessList(ctx, accessListName)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to get access list", "error", err)
	}

	var accessListTitle string
	if accessList != nil {
		accessListTitle = accessList.Spec.Title
	}
	for _, batch := range batchAccessListMemberMetadata(accessListName, accessListTitle, members) {
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
			s.logger.WarnContext(ctx, "Failed to emit access list member create/update event", "error", emitErr)
		}
	}
}

func (s *Service) emitUpsertAccessListMemberUsageEvent(ctx context.Context, updated bool, accessListName string, member *accesslist.AccessListMember) {
	if s.usageEvents == nil {
		return
	}

	var event *usageeventsv1.UsageEventOneOf
	memberMembershipKind := accesslistv1.MembershipKind_MEMBERSHIP_KIND_UNSPECIFIED
	if enum, ok := accesslistv1.MembershipKind_value[member.Spec.MembershipKind]; ok {
		memberMembershipKind = accesslistv1.MembershipKind(enum)
	}
	if updated {
		event = &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessListMemberUpdate{
				AccessListMemberUpdate: &usageeventsv1.AccessListMemberUpdate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: accessListName,
					},
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{
						MembershipKind: memberMembershipKind,
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
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{
						MembershipKind: memberMembershipKind,
					},
				},
			},
		}
	}
	if err := s.usageEvents.SubmitUsageEvent(ctx, &proto.SubmitUsageEventRequest{Event: event}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list member create/update usage event", "error", err)
	}
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (s *Service) DeleteAccessListMember(ctx context.Context, req *accesslistv1.DeleteAccessListMemberRequest) (*emptypb.Empty, error) {
	if err := validateMemberMetaRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := s.deleteAccessListMember(ctx, req, memberOptions{})
	return &emptypb.Empty{}, trace.Wrap(err)
}

// DeleteStaticAccessListMember hard deletes the specified access_list_member. It returns error and does
// nothing if the target access_list is not of static type.
func (s *Service) DeleteStaticAccessListMember(ctx context.Context, req *accesslistv1.DeleteStaticAccessListMemberRequest) (*accesslistv1.DeleteStaticAccessListMemberResponse, error) {
	if err := validateMemberMetaRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := s.deleteAccessListMember(ctx, req, memberOptions{
		requireStatic: true,
	})
	return &accesslistv1.DeleteStaticAccessListMemberResponse{}, trace.Wrap(err)
}

func (s *Service) deleteAccessListMember(ctx context.Context, req memberMetaGetter, opts memberOptions) error {
	authCtx, err := s.authOrIsOwner(ctx, req.GetAccessList(), types.VerbDelete)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	if opts.requireStatic {
		acl, err := s.accessLists.GetAccessList(ctx, req.GetAccessList())
		if err != nil {
			return trace.Wrap(err)
		}
		typ := acl.Spec.Type
		if typ != accesslist.Static {
			return trace.Wrap(newNonStaticAccessListErrorFromMemberMetaReq(req, typ))
		}
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.checkMembersModificationAllowedByName(ctx, *authCtx, req.GetAccessList()); err != nil {
		return trace.Wrap(err, "deleting members not allowed for Access List %q", req.GetAccessList())
	}

	deleteErr := s.accessLists.DeleteAccessListMember(ctx, req.GetAccessList(), req.GetMemberName())

	s.emitDeleteAccessListMemberEvent(ctx, username, req.GetAccessList(), deleteErr,
		accessListMembersForEvent(time.Time{}, s.clock.Now(), &memberEventMetadata{name: req.GetMemberName()})...)

	if deleteErr == nil {
		s.emitDeleteAccessListMemberUsageEvent(ctx, req.GetAccessList())
	}

	return trace.Wrap(deleteErr)
}

// emitDeleteAccessListMemberEvent will emit the delete event for the access list member.
func (s *Service) emitDeleteAccessListMemberEvent(ctx context.Context, username string, accessListName string, deleteErr error, members ...*apievents.AccessListMember) {
	var errorMsg string
	eventCode := events.AccessListMemberDeleteSuccessCode
	if deleteErr != nil {
		eventCode = events.AccessListMemberDeleteFailureCode
		errorMsg = deleteErr.Error()
	}

	accessList, err := s.cache.GetAccessList(ctx, accessListName)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to get access list", "error", err)
	}
	var accessListTitle string
	if accessList != nil {
		accessListTitle = accessList.Spec.Title
	}
	for _, batch := range batchAccessListMemberMetadata(accessListName, accessListTitle, members) {
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
			s.logger.WarnContext(ctx, "Failed to emit access list delete member event", "error", emitErr)
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
					// TODO(kiosion): Pass in metadata about the member being deleted.
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{},
				},
			},
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list delete usage event", "error", err)
	}
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list (without deleting the access list itself).
func (s *Service) DeleteAllAccessListMembersForAccessList(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authOrIsOwner(ctx, req.GetAccessList(), types.VerbDelete)
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

	if err := s.checkMembersModificationAllowedByName(ctx, *authCtx, req.GetAccessList()); err != nil {
		return nil, trace.Wrap(err, "deleting members not allowed for Access List %q", req.GetAccessList())
	}

	resp, deleteErr := s.deleteAllAccessListMembersForAccessList(ctx, req)

	s.emitDeleteAllAccessListMembersForAccessListEvent(ctx, username, req.GetAccessList(), deleteErr)

	return resp, trace.Wrap(deleteErr)
}

// deleteAllAccessListMembersForAccessList is a helper for deleting all access list members for an access list that returns the response and an error.
func (s *Service) deleteAllAccessListMembersForAccessList(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) (*emptypb.Empty, error) {
	err := s.accessLists.DeleteAllAccessListMembersForAccessList(ctx, req.GetAccessList())
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

	accessList, err := s.cache.GetAccessList(ctx, accessListName)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to get access list", "error", err)
	}
	var accessListTitle string
	if accessList != nil {
		accessListTitle = accessList.Spec.Title
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
			AccessListName:  accessListName,
			AccessListTitle: accessListTitle,
		},
		Status: apievents.Status{
			Success: deleteErr == nil,
			Error:   errorMsg,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list delete event", "error", emitErr)
	}
}

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (s *Service) UpsertAccessListWithMembers(ctx context.Context, req *accesslistv1.UpsertAccessListWithMembersRequest) (*accesslistv1.UpsertAccessListWithMembersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, updated, accessListModified, modifiedMembers, upsertErr := s.upsertAccessListWithMembers(ctx, authCtx, req)

	var accessListName string
	var title string
	if resp != nil {
		accessListName = resp.GetAccessList().GetHeader().GetMetadata().GetName()
		title = resp.GetAccessList().GetSpec().GetTitle()
	}
	if accessListModified {
		s.emitUpsertAccessListEvent(ctx, username, updated, accessListName,
			title, upsertErr)

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
				for i := range modifiedMembers.created {
					s.emitUpsertAccessListMemberUsageEvent(ctx, false, accessListName, modifiedMembers.created[i])
				}
			}
		}
		if len(modifiedMembers.updated) > 0 {
			s.emitUpsertAccessListMemberEvent(ctx, username, true, accessListName, upsertErr,
				accessListMembersForEvent(s.clock.Now(), time.Time{}, accessListMembersToMemberEventMetadata(modifiedMembers.updated)...)...,
			)

			if upsertErr == nil {
				for i := range modifiedMembers.updated {
					s.emitUpsertAccessListMemberUsageEvent(ctx, true, accessListName, modifiedMembers.updated[i])
				}
			}
		}
		if len(modifiedMembers.deleted) > 0 {
			s.emitDeleteAccessListMemberEvent(ctx, username, accessListName, upsertErr,
				accessListMembersForEvent(time.Time{}, s.clock.Now(), accessListMembersToMemberEventMetadata(modifiedMembers.deleted)...)...,
			)

			if upsertErr == nil {
				for range modifiedMembers.deleted {
					s.emitDeleteAccessListMemberUsageEvent(ctx, accessListName)
				}
			}
		}
	}

	// Return the updated access list and members.
	return resp, trace.Wrap(upsertErr)
}

// memberChanges will be used to house the exact modifications made to the members to emit and event later.
type memberChanges struct {
	created []*accesslist.AccessListMember
	updated []*accesslist.AccessListMember
	deleted []*accesslist.AccessListMember
}

// upsertAccessListWithMembers is a helper for upserting an access list with members that returns the response,
// whether the access list was updated, the modified members, and an error.
func (s *Service) upsertAccessListWithMembers(ctx context.Context, authCtx *authz.Context,
	req *accesslistv1.UpsertAccessListWithMembersRequest) (resp *accesslistv1.UpsertAccessListWithMembersResponse, updated,
	accessListModified bool, modified *memberChanges, err error,
) {
	newAccessList, err := conv.FromProto(req.GetAccessList())
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

	accessListModified = !accesslist.EqualAccessLists(oldAccessList, newAccessList, accesslist.WithIgnoreEphemeralFields())

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

	if accessListModified {
		if err := s.checkModificationAllowed(*authCtx, oldAccessList, newAccessList); err != nil {
			return nil, updated, accessListModified, nil, trace.Wrap(err)
		}
	}

	hasRBAC := authErrOld == nil && authErrNew == nil
	ownershipType, err := accesslists.IsAccessListOwner(ctx, authCtx.User, newAccessList, s.accessLists, s.lockGetter, s.clock)
	isOwner := err == nil && ownershipType != accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED

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
	oldMembers, err := s.getAccessListMemberMap(ctx, req.GetAccessList().GetHeader().GetMetadata().GetName())
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	username, err := getUsername(authCtx)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	// Convert members
	members := make([]*accesslist.AccessListMember, 0, len(req.GetMembers()))
	for _, member := range req.GetMembers() {
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

	if areMembersModified(oldMembers, members) {
		err = s.checkMembersModificationAllowed(ctx, *authCtx, oldAccessList)
		if trace.IsAccessDenied(err) {
			return nil, updated, accessListModified, nil, trace.Wrap(err, "forbidden Access List %q members modification", oldAccessList.GetName())
		} else if err != nil {
			return nil, updated, accessListModified, nil, trace.Wrap(err, "checking if Access List %q members modification is allowed", oldAccessList.GetName())
		}
	}

	// Call the API.
	updatedAccessList, updatedMembers, err := s.accessLists.UpsertAccessListWithMembers(ctx, newAccessList, members)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}

	// Figure out the member modifications for event emitting.
	modified = getMemberChanges(oldMembers, updatedMembers)

	// Get a list of all users, to compute eligibility's.
	users, err := getAllUsers(ctx, s.cache)
	if err != nil {
		return nil, updated, accessListModified, nil, trace.Wrap(err)
	}
	userLookup := makeUserLookup(users)

	updatedProtoMembers := applyMembersIneligibleStatus(updatedMembers, updatedAccessList.GetMembershipRequires(), s.clock, userLookup)
	updatedOwners := applyOwnersIneligibleStatus(updatedAccessList, s.clock, userLookup)
	updatedAccessList.SetOwners(updatedOwners)

	// Return the updated access list and members.
	return accesslistv1.UpsertAccessListWithMembersResponse_builder{
		AccessList: conv.ToProto(updatedAccessList),
		Members:    updatedProtoMembers,
	}.Build(), updated, accessListModified, modified, nil
}

// canUpdateMembership will return an error if the given user is unable to update membership for this member.
func (s *Service) canUpdateMembership(ctx context.Context, authCtx *authz.Context, username string,
	oldMember, newMember *accesslist.AccessListMember,
) error {
	err := s.userTryingToAddThemselves(ctx, authCtx, username, newMember)
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

	if !oldMember.IsEqual(newMember) {
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

func areMembersModified(oldMembers map[string]*accesslist.AccessListMember, newMembers []*accesslist.AccessListMember) bool {
	if len(newMembers) != len(oldMembers) {
		return true
	}
	for _, member := range newMembers {
		_, ok := oldMembers[member.Spec.Name]
		if !ok {
			return true
		}
	}

	return false
}

// getMemberChanges will get the modified members of the access list by comparing to the given old member map. If the old member
// map is nil, modified members will be nil.
//
// Caution: oldMembers list is modified in the process.
func getMemberChanges(oldMembers map[string]*accesslist.AccessListMember, updatedMembers []*accesslist.AccessListMember) *memberChanges {
	if oldMembers == nil {
		return nil
	}

	modified := &memberChanges{}
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
		s.logger.DebugContext(ctx, "hasAccessListRBAC had unexpected error", "error", authErr)
	}

	return trace.Wrap(authErr)
}

// hasUserRBAC tests if the user has RBAC access to users.
func (s *Service) hasUserRBAC(ctx context.Context, authCtx *authz.Context, verb string, additionalVerbs ...string) bool {
	authErr := authCtx.CheckAccessToKind(types.KindUser, verb, additionalVerbs...)
	if authErr != nil {
		s.logger.DebugContext(ctx, "hasUserRBAC had error", "error", authErr)
	}

	return authErr == nil
}

// isOwnerOfAccessList checks if this user owns this access list.
func (s *Service) isOwnerOfAccessList(ctx context.Context, authCtx *authz.Context, accessList *accesslist.AccessList) error {
	ownershipType, err := accesslists.IsAccessListOwner(ctx, authCtx.User, accessList, s.accessLists, s.lockGetter, s.clock)
	if err != nil {
		return trace.Wrap(err)
	}
	if ownershipType == accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
		return trace.AccessDenied("access denied")
	}

	return nil
}

// AccessRequestPromote promotes an access request to an access list.
func (s *Service) AccessRequestPromote(ctx context.Context, req *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error) {
	// Ensure current user is either owner or has permission to add members to the provided ACL.
	accessList, authCtx, err := s.authOrIsOwnerWithAccessList(ctx, req.GetAccessListName(), types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	accessReviewSubmission := types.AccessReviewSubmission{
		RequestID: req.GetRequestId(),
		Review: types.AccessReview{
			ProposedState: types.RequestState_PROMOTED,
			AccessList: &types.PromotedAccessList{
				Name:  req.GetAccessListName(),
				Title: accessList.Spec.Title,
			},
			Reason: req.GetReason(),
		},
	}

	// review author defaults to username of caller.
	if accessReviewSubmission.Review.Author == "" {
		accessReviewSubmission.Review.Author = authCtx.User.GetName()
	}

	// TODO(kiosion): Owners of a target list should be able to Promote (not approve/deny otherwise) to that list,
	// regardless of ReviewPermissionChecker's HasAllowDirectives
	if err := auth.AuthorizeAccessReviewRequest(*authCtx, accessReviewSubmission); err != nil {
		return nil, trace.Wrap(err)
	}

	accessReqs, err := s.authServer.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: req.GetRequestId(),
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
		return p.AccessListName == req.GetAccessListName()
	}) {
		return nil, trace.AccessDenied("access request cannot be promoted to requested access list")
	}

	memberName := accessReq.GetUser()

	// Submit review first; this validates that
	// a) the user can review this request, and
	// b) that the request is in a valid state for review.
	promotedAccessReq, err := s.authServer.SubmitAccessReview(ctx, accessReviewSubmission)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add ACL member after review submission.
	_, _, _, err = s.runAccessListMemberOp(ctx, authCtx, &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Kind:    types.KindAccessListMember,
			Version: types.V3,
			Metadata: header.Metadata{
				Name: memberName,
			},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList: req.GetAccessListName(),
			Name:       memberName,
			AddedBy:    authCtx.User.GetName(),
		},
	}, s.accessLists.UpsertAccessListMember)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessRequest, ok := promotedAccessReq.(*types.AccessRequestV3)
	if !ok {
		err = trace.BadParameter("unexpected access request type %T", req)
		return nil, trace.Wrap(err)
	}

	return accesslistv1.AccessRequestPromoteResponse_builder{
		AccessRequest: accessRequest,
	}.Build(), nil
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (s *Service) ListAccessListReviews(ctx context.Context, req *accesslistv1.ListAccessListReviewsRequest) (*accesslistv1.ListAccessListReviewsResponse, error) {
	if _, err := s.authOrIsOwner(ctx, req.GetAccessList(), types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	reviews, nextToken, err := s.accessListReviews.ListAccessListReviews(ctx, req.GetAccessList(), int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := accesslistv1.ListAccessListReviewsResponse_builder{
		Reviews:   make([]*accesslistv1.Review, len(reviews)),
		NextToken: nextToken,
	}.Build()

	for i, review := range reviews {
		resp.GetReviews()[i] = conv.ToReviewProto(review)
	}

	return resp, nil
}

// CreateAccessListReview will create a new review for an access list. It will also modify the original access list
// and its members depending on the details of the review.
func (s *Service) CreateAccessListReview(ctx context.Context, req *accesslistv1.CreateAccessListReviewRequest) (*accesslistv1.CreateAccessListReviewResponse, error) {
	review, err := conv.FromReviewProto(req.GetReview())
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

	accessList, err := s.accessLists.GetAccessList(ctx, req.GetReview().GetSpec().GetAccessList())
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
	// If the access list is from Entra ID, we don't allow removing members during review.
	if isEntraIDOrigin(accessList) {
		if len(review.Spec.Changes.RemovedMembers) > 0 {
			return nil, review, trace.BadParameter("membership changes are not allowed for access lists created via Entra ID integration")
		}
	}

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

	return accesslistv1.CreateAccessListReviewResponse_builder{
		ReviewName:    updatedReview.GetName(),
		NextAuditDate: timestamppb.New(nextAuditDate),
	}.Build(), updatedReview, nil
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

	accessList, err := s.cache.GetAccessList(ctx, review.Spec.AccessList)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to get access list", "error", err)
	}
	var accessListTitle string
	if accessList != nil {
		accessListTitle = accessList.Spec.Title
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
			AccessListTitle:               accessListTitle,
		},
		Status: apievents.Status{
			Success: createErr == nil,
			Error:   errorMsg,
		},
	}

	if emitErr := s.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit access list review create event", "error", emitErr)
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
		s.logger.WarnContext(ctx, "Failed to emit access list review create usage event", "error", err)
	}
}

// DeleteAccessListReview will delete an access list review from the backend.
func (s *Service) DeleteAccessListReview(ctx context.Context, req *accesslistv1.DeleteAccessListReviewRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ignore errors, we just want the access list if it exists for rbac purposes.
	accessList, _ := s.accessLists.GetAccessList(ctx, req.GetAccessListName())

	if err := s.hasAccessListRBAC(ctx, authCtx, accessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.accessListReviews.DeleteAccessListReview(ctx, req.GetAccessListName(), req.GetReviewName()); err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitDeleteAccessListReviewUsageEvent(ctx, req.GetAccessListName(), req.GetReviewName())

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
		s.logger.WarnContext(ctx, "Failed to emit access list review delete usage event", "error", err)
	}
}

// GetSuggestedAccessLists returns suggested access lists for an access request.
func (s *Service) GetSuggestedAccessLists(ctx context.Context, request *accesslistv1.GetSuggestedAccessListsRequest) (*accesslistv1.GetSuggestedAccessListsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity := authCtx.Identity.GetIdentity()
	suggestions, err := s.modules.GetSuggestedAccessLists(ctx, &identity, s.authServer, s.accessLists, request.GetAccessRequestId())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	suggestionsProto := make([]*accesslistv1.AccessList, len(suggestions))
	for i, r := range suggestions {
		suggestionsProto[i] = conv.ToProto(r)
	}

	return accesslistv1.GetSuggestedAccessListsResponse_builder{
		AccessLists: suggestionsProto,
	}.Build(), nil
}

// ListUserAccessLists returns a paginated list of all access lists where the
// user is explicitly an owner or member.
func (s *Service) ListUserAccessLists(ctx context.Context, req *accesslistv1.ListUserAccessListsRequest) (*accesslistv1.ListUserAccessListsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := s.authServer.GetUser(ctx, req.GetUsername(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userAcls, err := s.listAccessListsForUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filteredAcls, err := s.filterResults(ctx, userAcls, false, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	paginatedAcls, nextToken, totalCount, err := paginateSlice(filteredAcls, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslistv1.AccessList, len(paginatedAcls))
	for i, r := range paginatedAcls {
		accessLists[i] = conv.ToProto(r)
	}

	return accesslistv1.ListUserAccessListsResponse_builder{
		AccessLists:   accessLists,
		NextPageToken: nextToken,
		TotalCount:    int32(totalCount),
	}.Build(), nil
}

// listAccessListsForUser returns a slice of all access lists associated with
// the given user, including those inherited through hierarchical membership or
// ownership.
func (s *Service) listAccessListsForUser(ctx context.Context, user types.User) ([]*accesslist.AccessList, error) {
	h, err := accesslists.NewHierarchy(accesslists.HierarchyConfig{
		AccessListsService: s.accessLists,
		Clock:              s.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userAccessLists := make(map[string]*accesslist.AccessList)

	// process member/owner hierarchy and update aggregated acl assignments
	processAssignment := func(hierarchy []*accesslist.AccessList, accessListName string, isOwner bool) {
		for _, acl := range hierarchy {
			aclName := acl.GetName()

			var assignmentType accesslistv1.AccessListUserAssignmentType
			if aclName == accessListName {
				assignmentType = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT
			} else {
				assignmentType = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED
			}

			if _, exists := userAccessLists[aclName]; !exists {
				acl.Status.UserAssignments = &accesslist.UserAssignments{}
				userAccessLists[aclName] = acl
			}

			if isOwner {
				userAccessLists[aclName].Status.UserAssignments.OwnershipType = assignmentType
			} else {
				userAccessLists[aclName].Status.UserAssignments.MembershipType = assignmentType
			}
		}
	}

	iterFn := func(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
		req := accesslistv1.ListAccessListsV2Request_builder{
			PageSize:  int32(pageSize),
			PageToken: nextToken,
		}.Build()
		page, nextToken, err := s.cache.ListAccessListsV2(ctx, req)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return page, nextToken, nil
	}

	for acl, err := range clientutils.Resources(ctx, iterFn) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		memberOf, ownerOf, err := h.GetHierarchyForUser(ctx, acl, user)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// no explicit assignment
		if len(memberOf) == 0 && len(ownerOf) == 0 {
			continue
		}

		processAssignment(memberOf, acl.GetName(), false)
		processAssignment(ownerOf, acl.GetName(), true)
	}

	return slices.Collect(maps.Values(userAccessLists)), nil
}

// paginateSlice paginates a slice of access lists. It sorts by name and returns
// pageSize results starting from the access list whose name matches pageToken.
func paginateSlice(acls []*accesslist.AccessList, pageSize int, pageToken string) ([]*accesslist.AccessList, string, int, error) {
	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}

	slices.SortFunc(acls, func(a, b *accesslist.AccessList) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	pageStart := 0

	if pageToken != "" {
		for i, item := range acls {
			if item.GetName() == pageToken {
				pageStart = i
				break
			}
		}
	}

	pageEnd := pageSize + pageStart

	var nextToken string
	var err error
	if pageEnd >= len(acls) {
		pageEnd = len(acls)
	} else {
		nextToken, err = services.CreateAccessListNextKey(acls[pageEnd], "name")
		if err != nil {
			return nil, "", 0, trace.Wrap(err)
		}
	}

	totalCount := len(acls)
	results := acls[pageStart:pageEnd]

	return results, nextToken, totalCount, nil
}

// Check if the user is either authorized for the access list or owns this access list.
// Returns early if user has RBAC access (skips the step for retrieving an access list).
func (s *Service) authOrIsOwner(ctx context.Context, accessListName string, verb string, additionalVerbs ...string) (*authz.Context, error) {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.logger.DebugContext(ctx, "Failed to authorize user", "error", err)
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
		s.logger.DebugContext(ctx, "Failed to get access list", "error", getErr)
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
		s.logger.DebugContext(ctx, "Failed to authorize user", "error", err)
		// Return an opaque error
		return nil, nil, trace.AccessDenied("access denied")
	}

	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		s.logger.DebugContext(ctx, "Failed to get access list", "error", err)
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

// addMemberCounts to the given access list, if the user is not a member.
func (s *Service) addMemberCounts(ctx context.Context, isMember bool, accessList *accesslist.AccessList) {
	if isMember {
		return
	}

	memberCount, listCount, err := s.accessLists.CountAccessListMembers(ctx, accessList.GetName())
	if err != nil {
		s.logger.ErrorContext(ctx, "Error counting access list members", "error", err)
		return
	}
	accessList.Status.MemberCount = &memberCount
	accessList.Status.MemberListCount = &listCount
}

func (s *Service) runAccessListIneligibleReconciler(ctx context.Context) error {
	const accessListIneligibleReconciler = "access_list_ineligible_reconciler"
	for {
		err := backend.RunWhileLocked(
			ctx,
			backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					LockNameComponents: []string{accessListIneligibleReconciler},
					Backend:            s.backend,
					TTL:                60 * time.Second,
					RetryInterval:      30 * time.Second,
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
						Logger:  s.logger.With("reconciler", accessListIneligibleReconciler),
						Clock:   s.clock,
					},
				)
				if err != nil {
					return trace.Wrap(err)
				}
				defer reconciler.Close()
				if err := reconciler.Run(ctx); err != nil {
					s.logger.ErrorContext(ctx, "Error running access list ineligible reconciler", "error", err)
					return trace.Wrap(err)
				}
				return nil
			},
		)
		if err != nil {
			if ctx.Err() != nil {
				return trace.Wrap(ctx.Err())
			}
			s.logger.ErrorContext(ctx, "Error running access list ineligible reconciler", "error", err)
			select {
			case <-s.clock.After(30 * time.Second):
			case <-ctx.Done():
				return trace.Wrap(err)
			}
		}
	}
}

func (s *Service) runAccessListStatusReconciler(ctx context.Context) error {
	const accessListStatusReconciler = "access_list_status_reconciler"
	for {
		err := backend.RunWhileLocked(
			ctx,
			backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					LockNameComponents: []string{accessListStatusReconciler},
					Backend:            s.backend,
					TTL:                60 * time.Second,
					RetryInterval:      30 * time.Second,
				},
				ReleaseCtxTimeout:   60 * time.Second,
				RefreshLockInterval: 30 * time.Second,
			},
			func(ctx context.Context) error {
				reconciler, err := newStatusReconciler(statusReconcilerConfig{
					Logger:      s.logger.With("reconciler", accessListStatusReconciler),
					Clock:       s.clock,
					AccessPoint: s.accessLists,
				})
				if err != nil {
					return trace.Wrap(err)
				}
				return trace.Wrap(reconciler.Run(ctx))
			},
		)
		if err != nil {
			s.logger.ErrorContext(ctx, "Error running access list status reconciler", "error", err)
			select {
			case <-s.clock.After(30 * time.Second):
			case <-ctx.Done():
				return trace.Wrap(err)
			}
		}
	}
}

// checkModificationAllowed returns AccessDenied error if modifications applied from the old to the
// new Access List are not allowed. It can return any other error.
func (s *Service) checkModificationAllowed(authCtx authz.Context, oldAccessList, newAccessList *accesslist.AccessList) error {
	if !oktaModificationAllowed(authCtx, oldAccessList, newAccessList) {
		return trace.AccessDenied("Okta sourced Access Lists cannot be modified")
	}

	return nil
}

// checkMembersModificationAllowedByName returns AccessDenied if Access List members' modifications are
// not allowed. It can return any other error.
func (s *Service) checkMembersModificationAllowedByName(ctx context.Context, authCtx authz.Context, accessListName string) error {
	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		return trace.Wrap(err, "getting Access List")
	}

	err = s.checkMembersModificationAllowed(ctx, authCtx, accessList)
	return trace.Wrap(err)
}

func (s *Service) checkAccessListDeletionAllowed(ctx context.Context, authCtx authz.Context, accessList *accesslist.AccessList) error {
	allowed, err := oktaDeletionAllowed(ctx, authCtx, s.plugins, accessList)
	if err != nil {
		return trace.Wrap(err)
	}

	if !allowed {
		return trace.AccessDenied("Unable to delete Okta-originated Access List")
	}

	return nil
}

func isEntraIDOrigin(accessList *accesslist.AccessList) bool {
	if accessList == nil {
		return false
	}
	origin, ok := accessList.Metadata.GetLabel(types.OriginLabel)
	return ok && origin == types.OriginEntraID
}

// checkMembersModificationAllowed returns AccessDenied if Access List members' modifications are
// not allowed. It can return any other error.
func (s *Service) checkMembersModificationAllowed(ctx context.Context, authCtx authz.Context, accessList *accesslist.AccessList) error {
	if accessList != nil && accessList.Spec.Type == accesslist.SCIM {
		return trace.BadParameter("SCIM-sourced Access List members modification not allowed")
	}

	if isEntraIDOrigin(accessList) {
		return trace.BadParameter("Entra ID-sourced Access List members modification not allowed")
	}

	if allowed, err := oktaMembersModificationAllowed(ctx, authCtx, s.plugins, accessList); err != nil {
		return trace.Wrap(err, "running Okta-specific member modification checks")
	} else if !allowed {
		return trace.BadParameter("Okta-sourced Access List members modification not allowed when bidirectional sync is disabled")
	}
	return nil
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
	ownerMeetsRequirements := accesslists.UserMeetsRequirements(foundUser, f.requires)
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
	kind     accesslistv1.MembershipKind
	reason   string
	joinedOn time.Time
}

// accessListMemberProtoToMemberEventMetadata converts a member proto into a memberNameAndReason.
func accessListMemberProtoToMemberEventMetadata(member *accesslistv1.Member) *memberEventMetadata {
	if member.GetSpec() == nil {
		return nil
	}

	return &memberEventMetadata{
		name:     member.GetSpec().GetName(),
		kind:     member.GetSpec().GetMembershipKind(),
		reason:   member.GetSpec().GetReason(),
		joinedOn: member.GetSpec().GetJoined().AsTime(),
	}
}

// accessListMembesrToMemberEventMetadata converts all members into a memberNameAndReason.
func accessListMembersToMemberEventMetadata(members []*accesslist.AccessListMember) []*memberEventMetadata {
	convertedMembers := []*memberEventMetadata{}
	for _, member := range members {
		if member == nil {
			return nil
		}

		kind := accesslistv1.MembershipKind_MEMBERSHIP_KIND_UNSPECIFIED
		if enum, ok := accesslistv1.MembershipKind_value[member.Spec.MembershipKind]; ok {
			kind = accesslistv1.MembershipKind(enum)
		}

		convertedMembers = append(convertedMembers, &memberEventMetadata{
			name:     member.Spec.Name,
			kind:     kind,
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
			JoinedOn:       joinedOn,
			RemovedOn:      removeTime,
			Reason:         member.reason,
			MemberName:     member.name,
			MembershipKind: member.kind,
		}

		eventMembers = append(eventMembers, eventMember)
	}

	return eventMembers
}

// batchAccessListMemberMetadata will create batches of access list member metadata objects for emitting events in batches.
func batchAccessListMemberMetadata(accessListName string, accessListTitle string, members []*apievents.AccessListMember) []apievents.AccessListMemberMetadata {
	numMembers := len(members)
	numBatches := int(math.Ceil(float64(numMembers) / float64(eventMemberBatches)))
	batches := make([]apievents.AccessListMemberMetadata, numBatches)

	for i := range numBatches {
		startIndex := i * eventMemberBatches
		endIndex := min(startIndex+eventMemberBatches, numMembers)
		batches[i] = apievents.AccessListMemberMetadata{
			AccessListName:  accessListName,
			Members:         members[startIndex:endIndex],
			AccessListTitle: accessListTitle,
		}
	}

	return batches
}

func getUsername(authCtx *authz.Context) (string, error) {
	if authCtx == nil {
		return "", trace.BadParameter("authCtx is nil")
	}

	if authz.HasBuiltinRole(*authCtx, string(types.RoleOkta)) {
		return OktaServiceRoleUsername, nil
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

type memberOptions struct {
	// requireStatic forces the operation to fail if the AccessList.Spec.Type is not "static".
	requireStatic bool
}

type memberMetaGetter interface {
	// GetAccessList returns the name of the access_list that the member belongs to.
	GetAccessList() string
	// GetMemberName returns the name of the user that belongs to the access_list.
	GetMemberName() string
}

type memberGetter interface {
	// GetMember returns the access_list_member.
	GetMember() *accesslistv1.Member
}

func (s *Service) checkCreateAccessListPresetPermissions(authCtx *authz.Context) error {
	if err := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbCreate, types.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRole, types.VerbCreate, types.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Service) checkUpdateAccessListPresetPermissions(authCtx *authz.Context) error {
	if err := authCtx.CheckAccessToKind(types.KindAccessList, types.VerbUpdate, types.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRole, types.VerbCreate, types.VerbUpdate, types.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateAccessListWithPreset creates access list preset.
func (s *Service) CreateAccessListWithPreset(ctx context.Context, req *accesslistv1.CreateAccessListWithPresetRequest) (*accesslistv1.CreateAccessListWithPresetResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkCreateAccessListPresetPermissions(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(req.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessRoles := make([]types.Role, 0, len(req.GetRoles()))
	for _, role := range req.GetRoles() {
		accessRoles = append(accessRoles, role)
	}

	roleBuilder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessList.GetName(),
		AccessListSpec: accessList,
		PresetType:     preset.PresetType(req.GetPresetType()),
		AccessRoles:    accessRoles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleBuildResult, err := roleBuilder.Build()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(smallinsky) implement proper access list creation flow instead of abusing Upsert calls
	// when the CreateAccessList method will be available.
	_, err = s.GetAccessList(ctx, accesslistv1.GetAccessListRequest_builder{Name: roleBuildResult.AccessList.GetName()}.Build())
	switch {
	case err == nil:
		return nil, trace.AlreadyExists("access list %v already exists", roleBuildResult.AccessList.GetName())
	case !trace.IsNotFound(err):
		return nil, trace.Wrap(err)
	default:
		// expect that access list not exists.
	}
	acl, err := s.UpsertAccessList(ctx, accesslistv1.UpsertAccessListRequest_builder{
		AccessList: conv.ToProto(roleBuildResult.AccessList),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.createBuildResultRoles(ctx, roleBuildResult); err != nil {
		return nil, trace.Wrap(err)
	}
	outRoles, err := castToRoleV6(roleBuildResult.AccessRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accesslistv1.CreateAccessListWithPresetResponse_builder{
		AccessList: acl,
		Roles:      outRoles,
	}.Build(), nil
}

// UpdateAccessListWithPreset updates existing access list preset.
func (s *Service) UpdateAccessListWithPreset(ctx context.Context, req *accesslistv1.UpdateAccessListWithPresetRequest) (*accesslistv1.UpdateAccessListWithPresetResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkUpdateAccessListPresetPermissions(authCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(req.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessRoles := make([]types.Role, 0, len(req.GetRoles()))
	for _, role := range req.GetRoles() {
		accessRoles = append(accessRoles, role)
	}

	builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessList.GetName(),
		AccessListSpec: accessList,
		PresetType:     preset.PresetType(accessList.GetAllLabels()[preset.TeleportAccessListPreset]),
		AccessRoles:    accessRoles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buildResult, err := builder.Build()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := s.UpdateAccessList(ctx, accesslistv1.UpdateAccessListRequest_builder{
		AccessList: conv.ToProto(buildResult.AccessList),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.upsertBuildResultRoles(ctx, buildResult); err != nil {
		return nil, trace.Wrap(err)
	}
	outRoles, err := castToRoleV6(buildResult.AccessRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accesslistv1.UpdateAccessListWithPresetResponse_builder{
		AccessList:       resp,
		Roles:            outRoles,
		RolesToBeDeleted: buildResult.RolesToBeDeleted,
	}.Build(), nil
}

func (s *Service) upsertBuildResultRoles(ctx context.Context, in *preset.BuildResult) error {
	for _, r := range in.GetAllRoles() {
		// TODO(smallinsky): Ideally, this should be handled in  transactions
		// that also includes access list creation. Currently, this flow mirrors
		// the Terraform approach, where access lists and roles are updated directly
		// via Upsert calls without an translation steps.
		if r.GetRevision() == "" {
			if _, err := s.authServer.UpsertRole(ctx, r); err != nil {
				return trace.Wrap(err)
			}
			continue
		}
		if _, err := s.authServer.UpdateRole(ctx, r); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Service) createBuildResultRoles(ctx context.Context, in *preset.BuildResult) error {
	for _, r := range in.GetAllRoles() {
		// TODO(smallinsky): Ideally, this should be handled in a transactions
		// that also includes access list creation. Currently, this flow mirrors
		// the Terraform approach, where access lists and roles are created directly
		// via Upsert calls without translation steps.
		if _, err := s.authServer.CreateRole(ctx, r); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func castToRoleV6(in []types.Role) ([]*types.RoleV6, error) {
	out := make([]*types.RoleV6, 0, len(in))
	for _, v := range in {
		v6, ok := v.(*types.RoleV6)
		if !ok {
			return nil, trace.BadParameter("expected RoleV6")
		}
		out = append(out, v6)
	}
	return out, nil
}
