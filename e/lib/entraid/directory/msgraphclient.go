package directory

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// GraphClient is an interface for interacting with the Microsoft Graph API
// using the lib/msgraph sdk.
type GraphClient interface {
	IterateUsers(ctx context.Context, f func(*models.User) bool, opts ...msgraph.IterateOpt) error
	IterateGroups(ctx context.Context, f func(*models.Group) bool, opts ...msgraph.IterateOpt) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(models.GroupMember) bool, opts ...msgraph.IterateOpt) error
	IterateGroupOwners(ctx context.Context, groupID string, f func(*models.User) bool, opts ...msgraph.IterateOpt) error
	IterateApplications(ctx context.Context, f func(*models.Application) bool, opts ...msgraph.IterateOpt) error
	GetApplication(ctx context.Context, appID string) (*models.Application, error)

	IterateUserDeltas(ctx context.Context, endpoint string, ds msgraph.DeltaStore) iter.Seq2[*models.ListUsersDeltaResponse, error]
	IterateGroupDeltas(ctx context.Context, endpoint string, ds msgraph.DeltaStore) iter.Seq2[*models.ListGroupsDeltaResponse, error]
	SetupLatestDelta(ctx context.Context, endpoint string, ds msgraph.DeltaStore, opts ...msgraph.IterateOpt) error
}

const (
	usersDeltaEndpoint  = "users/delta"
	groupsDeltaEndpoint = "groups/delta"
)

// deltaStore implements msgraph.DeltaStore
type deltaStore struct {
	mu    sync.Mutex
	cache map[string]string
}

func newDeltaStore() *deltaStore {
	return &deltaStore{
		cache: make(map[string]string),
	}
}

// Get gets delta link for the given endpoint.
func (s *deltaStore) Get(endpoint string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[endpoint]
}

// Set sets delta link for the given endpoint.
func (s *deltaStore) Set(endpoint, link string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[endpoint] = link
}

// Clear clears delta link for the given endpoint.
func (s *deltaStore) Clear(endpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, endpoint)
}

// graphClient is the graph client used by the directory reconciler.
type graphClient struct {
	GraphClient
	deltaStore          msgraph.DeltaStore
	setEntraGroupOwners bool
	graphClientLimit    graphClientLimit
	log                 *slog.Logger
}

// graphClientConfig is the configuration params to create
// a new graph client used by the directory reconciler.
type graphClientConfig struct {
	GraphClient
	deltaStore             msgraph.DeltaStore
	accessListOwnersSource types.EntraIDAccessListOwnersSource
	log                    *slog.Logger
}

func newGraphClient(cfg graphClientConfig) *graphClient {
	setEntraOwners := cfg.accessListOwnersSource == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID ||
		cfg.accessListOwnersSource == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID

	return &graphClient{
		GraphClient:         cfg.GraphClient,
		deltaStore:          cfg.deltaStore,
		setEntraGroupOwners: setEntraOwners,
		graphClientLimit:    limitHigh, // start with full limit.
		log:                 cfg.log,
	}
}

// backupDeltaLinks returns a callback function to restore delta links.
func (c graphClient) backupDeltaLinks() func() {
	userDeltaLinkBackup := c.deltaStore.Get(usersDeltaEndpoint)
	groupDeltaLinkBackup := c.deltaStore.Get(groupsDeltaEndpoint)

	return func() {
		c.deltaStore.Set(usersDeltaEndpoint, userDeltaLinkBackup)
		c.deltaStore.Set(groupsDeltaEndpoint, groupDeltaLinkBackup)
	}
}

type listEntraGroupsResponse struct {
	groupsMap groupsByID
	// errSkippedGroups is an error collection of failed group validation.
	// These errors should not stop the sync and instead should be reported
	// via the plugin status.
	errSkippedGroups []error
}

type listEntraGroupsAndMembersResponse struct {
	listEntraGroupsResponse
	groupMembersMap groupMembersByGroupID
}

func (c *graphClient) listEntraGroups(
	ctx context.Context,
	filterMatches func(g *models.Group) bool,
	accessListOwnersSource types.EntraIDAccessListOwnersSource,
) (listEntraGroupsResponse, error) {
	var errSkippedGroups []error
	result := groupsByID{}
	err := c.IterateGroups(ctx, func(g *models.Group) bool {
		if err := validateGroup(g); err != nil {
			errSkippedGroups = append(errSkippedGroups, trace.Wrap(err))
			return true
		}
		if filterMatches(g) {
			c.setEntraOwners(ctx, accessListOwnersSource, g)

			result[entraUniqueID(*g.ID)] = g
		}

		// defaults to true so the iteration continues.
		return true
	})

	resp := listEntraGroupsResponse{
		groupsMap:        result,
		errSkippedGroups: errSkippedGroups,
	}
	return resp, trace.Wrap(err)
}

func (c *graphClient) setEntraOwners(
	ctx context.Context,
	accessListOwnersSource types.EntraIDAccessListOwnersSource,
	g *models.Group) {
	if accessListOwnersSource == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID ||
		accessListOwnersSource == types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID {

		entraOwners, err := c.listEntraGroupOwners(ctx, *g.GetID())
		if err != nil {
			// error swallowed to let the sync continue.
			c.log.WarnContext(ctx, "Error while fetching Entra ID group owners", "group_id", g.GetID(), "error", err)
			return
		}
		g.Owners = entraOwners
	}
}

func (c *graphClient) listEntraGroupOwners(
	ctx context.Context,
	groupID string,
) ([]*models.User, error) {
	var owners []*models.User
	if err := c.IterateGroupOwners(ctx, groupID, func(o *models.User) bool {
		owners = append(owners, o)
		return true
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return owners, nil
}

func (c *graphClient) listEntraGroupsMembers(
	ctx context.Context,
	groups groupsByID,
) (groupMembersByGroupID, error) {

	members, err := fetchGroupMembers(ctx, groups, c)
	if err != nil {
		// RetryAfter duration swallowed below, which will
		// be separately handled by the Entra ID service.
		if _, throttled := IsErrGraphAPIThrottled(err); throttled {
			// Next sync will run on limitLow.
			c.graphClientLimit = limitLow
		}
		return nil, trace.Wrap(err)
	}

	// Upgrade goroutine limit.
	// There isn't any explicit cooldown period tracked here before upgrading because:
	// - The next sync happens after x interval (baked-in cooldown period)
	// - graph client itself does five retries honoring retry-after header.
	if c.graphClientLimit == limitLow {
		c.graphClientLimit = limitHigh
	}

	return members, nil
}

func fetchGroupMembers(
	ctx context.Context,
	groups groupsByID,
	c *graphClient,
) (groupMembersByGroupID, error) {
	// membersPageSize is the maximum number of members to fetch per page.
	// https://learn.microsoft.com/en-us/graph/api/group-list-members?view=graph-rest-1.0&tabs=http#http-request
	// We don't want to send 9 requests to fetch 900 members where 999 is max page size supported by API
	const membersPageSize = 300

	result := make(groupMembersByGroupID, len(groups))
	var mu sync.Mutex

	// TODO(smallinsky) move to static goroutine workers to not allocate space for each goroutine.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(getParallelReqCount(len(groups), c.graphClientLimit))
	for id, group := range groups {
		id, gid := id, *group.ID
		g.Go(func() error {
			var members []models.GroupMember
			if err := c.IterateGroupMembers(ctx, gid, func(m models.GroupMember) bool {
				members = append(members, m)
				return true
			}, msgraph.WithTop(membersPageSize)); err != nil {
				return trace.Wrap(err)
			}

			mu.Lock()
			result[id] = members
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return result, nil
}

// getParallelReqCount returns the number of parallel requests to use based on the number of groups.
// We want to balance and not run 30 parallel request for 50 groups.
// But with large dataset like 10k we want to have enough parallelism to not take hours to fetch all members.
func getParallelReqCount(numGroups int, limit graphClientLimit) int {
	switch limit {
	case limitHigh:
		if numGroups < 1000 {
			return 10
		}
		// 30 parallel request takes around 11-12 minutes With 70k groups and 100 to 5k+ group member each.
		return 30
	default:
		return 1
	}
}

func validateGroup(in *models.Group) error {
	if in == nil {
		return trace.BadParameter("expected Entra ID group to be non-nil")
	}
	if in.DisplayName == nil {
		return trace.BadParameter("expected Entra ID group%s to have a non-empty display name", groupNameForLog(in))
	}
	if in.ID == nil {
		return trace.BadParameter("expected Entra ID group%s to have a non-empty ID", groupNameForLog(in))
	}

	return nil
}

type listEntraUsersResponse struct {
	users map[string]types.User
	// errSkippedUsers is an error collection of failed user validation.
	// These errors should not stop the sync and instead should be reported
	// via the plugin status.
	errSkippedUsers []error
}

func (c *graphClient) listEntraUsers(
	ctx context.Context,
	cfg entraUserSyncCfg,
) (listEntraUsersResponse, error) {
	result := map[string]types.User{}
	var errSkippedUsers []error
	var unsupportedUsers []string

	err := c.IterateUsers(ctx, func(u *models.User) bool {
		user, err := convertUser(u, cfg.usersMemberships, cfg.userConfig)
		if err != nil {
			if errors.Is(err, errUnsupportedUsername) {
				unsupportedUsers = append(unsupportedUsers, unameForLog(u))
			} else {
				errSkippedUsers = append(errSkippedUsers, trace.Wrap(err))
			}
			return true
		}
		result[user.GetName()] = user
		return true
	})
	postProcess := postProcessUser{
		teleportUsers: cfg.teleportUsers,
		tms:           cfg.userConfig.tms,
	}
	postProcess.apply(result)

	if len(unsupportedUsers) > 0 {
		errSkippedUsers = append(errSkippedUsers, errUnsupportedUsers(unsupportedUsers))
	}
	resp := listEntraUsersResponse{
		users:           result,
		errSkippedUsers: errSkippedUsers,
	}
	return resp, trace.Wrap(err)
}

func processUsername(in *models.User) (string, bool, error) {
	if in == nil {
		return "", false, trace.BadParameter("expected Entra ID user to be non-nil")
	}
	upn := in.UserPrincipalName
	if upn == nil {
		return "", false, trace.BadParameter("expected Entra ID user to have a UPN")
	}

	username := in.Mail
	if username == nil {
		username = upn
	}

	isExternal := false
	// Entra ID  users may have a suffix that indicates they are external users in B2B Guest scenarios.
	// This suffix is removed when the user logins via the SAML assertion so we need to remove it here.
	// more info: https://docs.microsoft.com/en-us/azure/active-directory/external-identities/what-is-b2b
	// and https://learn.microsoft.com/en-us/entra/identity/app-provisioning/how-provisioning-works
	// Example: "user_theirdomain#EXT#@domain -> "user@theirdomain"
	const externalUserSuffix = "#EXT#"
	if idx := strings.Index(*username, externalUserSuffix); idx != -1 {
		// Reformat Entra ID external username [username_domain.com#EXT#@yourtenant.onmicrosoft.com]
		// to a Teleport supported username format [username@domain.com].
		user := (*username)[:idx] // remove #EXT#@domain
		if idx := strings.LastIndex(user, "_"); idx != -1 {
			*username = user[:idx] + "@" + user[idx+1:] // replace the last _ with @
		}
		isExternal = true
	}

	if err := isValidUsername(*username); err != nil {
		return "", false, trace.Wrap(err)
	}

	return *username, isExternal, nil
}

func isValidUsername(in string) error {
	// Username value is used as a backend key for the user resource.
	// Entra ID user's username may contain an unsupported characters such
	// as single quote ('), forward slash (/) etc and will cause the users
	// reconciler to fail.
	key := backend.NewKey(in)
	if !backend.IsKeySafe(key) {
		return errUnsupportedUsername
	}

	return nil
}

// setupUserAndGroupDelta configures latest delta token from msgraph
// user and group delta API. Should always be called before iterating
// over user and group delta API.
func (c *graphClient) setupUserAndGroupDelta(ctx context.Context) error {
	// Ensure these properties are always included.
	// A delta token, including the initial and subsequent tokens,
	// encodes all the queries that are requested in the
	// first delta query, i.e. when a "latest" token is asked.
	userOpts := []msgraph.IterateOpt{
		msgraph.WithSelect("id,displayName,userPrincipalName,mail,onPremisesSamAccountName,givenName,surname"),
	}

	groupProperties := "id,displayName,description,onPremisesSamAccountName,onPremisesDomainName,onPremisesNetBiosName,members"
	if c.setEntraGroupOwners {
		groupProperties += ",owners"
	}
	groupOpts := []msgraph.IterateOpt{
		msgraph.WithSelect(groupProperties),
	}

	// Only a single page of delta token is expected.
	if err := c.SetupLatestDelta(ctx, usersDeltaEndpoint, c.deltaStore, userOpts...); err != nil {
		return trace.Wrap(err, "setting up user latest delta token")
	}
	if err := c.SetupLatestDelta(ctx, groupsDeltaEndpoint, c.deltaStore, groupOpts...); err != nil {
		return trace.Wrap(err, "setting up group latest delta token")
	}

	return nil
}

type entraUserSyncCfg struct {
	userConfig       userConfig
	usersMemberships groupMembershipMap
	teleportUsers    map[string]types.User
}

// listEntraUsersDelta returns Entra ID users by processing over
// user delta response.
func (c *graphClient) listEntraUsersDelta(
	ctx context.Context,
	cfg entraUserSyncCfg,
) (listEntraUsersResponse, error) {

	var out listEntraUsersResponse
	var errSkipped []error
	var unsupportedUsers []string

	deltaProcessor := newUserDeltaProcessor(cfg.usersMemberships, cfg.teleportUsers, cfg.userConfig)

	for userDelta, err := range c.IterateUserDeltas(ctx, usersDeltaEndpoint, c.deltaStore) {
		if err != nil {
			return listEntraUsersResponse{}, trace.Wrap(err, "processing user deltas")
		}

		if err := deltaProcessor.apply(userDelta); err != nil {
			if errors.Is(err, errUnsupportedUsername) {
				unsupportedUsers = append(unsupportedUsers, unameForLog(userDelta.User))
			} else {
				errSkipped = append(errSkipped, trace.Wrap(err))
			}
			continue
		}
	}

	result := deltaProcessor.result()
	postProcess := postProcessUser{
		teleportUsers: cfg.teleportUsers,
		tms:           cfg.userConfig.tms,
	}
	postProcess.apply(result)
	out.users = result

	if len(unsupportedUsers) > 0 {
		errSkipped = append(errSkipped, errUnsupportedUsers(unsupportedUsers))
	}
	out.errSkippedUsers = errSkipped
	return out, nil
}

// listEntraGroupsDelta handles changes received in group, group owners and group members.
func (c *graphClient) listEntraGroupsDelta(
	ctx context.Context,
	groupMatcher func(g *models.Group) bool,
	accessListsMap map[string]*accessListWithMembers,
	teleportUsersMap map[string]types.User,
) (listEntraGroupsAndMembersResponse, error) {
	var out listEntraGroupsAndMembersResponse

	cfg := groupDeltaProcessorConfig{
		matcher:             groupMatcher,
		accessListsMap:      accessListsMap,
		teleportUsersMap:    teleportUsersMap,
		setEntraGroupOwners: c.setEntraGroupOwners,
		log:                 c.log,
	}
	deltaProcessor := newGroupsDeltaProcessor(ctx, cfg)

	var errSkipped []error
	for groupDelta, err := range c.IterateGroupDeltas(ctx, groupsDeltaEndpoint, c.deltaStore) {
		if err != nil {
			return out, trace.Wrap(err, "failed iterating over group deltas")
		}

		if err := deltaProcessor.apply(ctx, groupDelta); err != nil {
			errSkipped = append(errSkipped, err)
		}
	}

	result := deltaProcessor.result()
	out.groupsMap = result.groupsMap
	out.groupMembersMap = result.groupMembersMap
	out.errSkippedGroups = errSkipped
	return out, nil
}

// graphClientLimit defines goroutine limit value
// to be used in the parallel Graph API calls.
type graphClientLimit int

const (
	limitHigh graphClientLimit = iota
	limitLow
)
