package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v70/github"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type fetcher struct {
	// client is the HTTP client to use for making requests.
	client *github.Client
	// logger is the logger to use for logging.
	logger *slog.Logger
	// clock is the clock to use for getting the current time.
	clock clockwork.Clock
	// enterprise is true if the client is for an enterprise installation.
	enterprise bool
	// organizationName is the name of the organization to fetch data for.
	organizationName string
}

// newFetcher creates a new fetcher for the given configuration.
func newFetcher(
	logger *slog.Logger,
	clock clockwork.Clock,
	conf GithubConfig,
) (*fetcher, error) {
	githubClient := github.NewClient(&http.Client{
		Transport: &roundTripper{
			transport:        http.DefaultTransport,
			clientID:         conf.ClientID,
			organizationName: conf.Organization,
			privateKeyData:   conf.PrivateKey,
		},
	})

	enterprise := false
	if conf.Address != "" {
		var err error
		githubClient, err = githubClient.WithEnterpriseURLs(conf.Address, "")
		if err != nil {
			return nil, trace.Wrap(err, "failed to create github client with enterprise url")
		}
		enterprise = true
	}

	return &fetcher{
		client:           githubClient,
		logger:           logger,
		clock:            clock,
		enterprise:       enterprise,
		organizationName: conf.Organization,
	}, nil
}

// auditLogCursor is used to keep track of the last audit log entry that was
// processed. It contains the token used for pagination, the last ID of the
// processed entry, and the timestamp of the last processed entry.
type auditLogCursor struct {
	// token is the pagination token used to fetch the next page of audit logs.
	// It is set to the value of the "After" field in the response.
	token string
	// lastID is the ID of the last processed audit log entry. It is used to
	// determine the starting point for the next poll.
	// It is set to the value of the "DocumentID" field in the response if
	// the "After" field is not set - this means that there are no more pages to
	// fetch.
	lastID string
	// lastTimestamp is the timestamp of the last processed audit log entry.
	lastTimestamp time.Time
}

// pollAuditLogs fetches audit logs from GitHub starting from the given
// timestamp and cursor. It returns the fetched audit logs, the updated cursor,
// a boolean indicating whether there are more logs to fetch, and an error if
// any occurred.
func (f *fetcher) pollAuditLogs(ctx context.Context, since time.Time, startCursor auditLogCursor) ([]*structpb.Struct, auditLogCursor, bool, error) {
	const (
		// includeAllEvents is the value for the "include" field in the request.
		// It specifies which events to include in the response.
		// "all" means git events and org events.
		includeAllEvents = "all"
		// order is the value for the "order" field in the request.
		order = "asc"
		// maxEventsPerPoll is the maximum number of events to fetch per poll.
		maxEventsPerPoll = 100
		// maxElemSize is the maximum number of elements to fetch sequentially.
		maxElemSize = 5000
		// maxMemorySize is the maximum size of the fetched events in memory.
		maxMemorySize = 3 * 1024 * 1024 // 3MB
	)

	opt := github.GetAuditLogOptions{
		Phrase: toPtr(
			fmt.Sprintf("created:%s..%s",
				since.UTC().Format(time.RFC3339),
				f.clock.Now().UTC().Format(time.RFC3339),
			),
		),
		Include: toPtr(includeAllEvents),
		ListCursorOptions: github.ListCursorOptions{
			PerPage: maxEventsPerPoll,
			After:   startCursor.token,
		},
		Order: toPtr(order),
	}

	var (
		githubRateLimitError *github.RateLimitError
		size                 int
	)

	events := make([]*structpb.Struct, 0, maxElemSize)

	// Use the appropriate function to get the audit log based on whether
	// the client is for an enterprise installation or not.
	getFunc := f.client.Enterprise.GetAuditLog
	if !f.enterprise {
		getFunc = f.client.Organizations.GetAuditLog
	}

	for {
		entries, resp, err := getFunc(ctx, f.organizationName, &opt)
		switch {
		case errors.As(err, &githubRateLimitError):
			f.logger.DebugContext(ctx, "Github rate limit error", "error", err)
			return events, startCursor, false, trace.Wrap(err)
		case err != nil:
			return events, startCursor, false, trace.Wrap(err)
		case resp.StatusCode != http.StatusOK:
			return events, startCursor, false, trace.Errorf("API error fetching GitHub audit logs, status_code: %d",
				resp.StatusCode)
		}

		if startCursor.lastID != "" {
			entries = skipProcessedEntries(entries, startCursor.lastID)
			// If the lastID was the last entry in the list, we need to
			// set the token to the next page token.
			if len(entries) == 0 && resp.After != "" {
				opt.After = resp.After
				startCursor.token = resp.After
				startCursor.lastID = ""
				continue
			}
		}

		if len(entries) == 0 {
			f.logger.DebugContext(ctx, "No audit log entries found")
			break
		}

		for _, entry := range entries {
			pbStruct, err := auditToProtoStruct(entry)
			if err != nil {
				return events, startCursor, false, trace.Wrap(err)
			}
			events = append(events, pbStruct)
			size += proto.Size(pbStruct)
		}

		f.logger.DebugContext(ctx, "Received system logs from GitHub", "log_count", len(entries))
		if resp.After == "" {
			startCursor.lastID = entries[len(entries)-1].GetDocumentID()
			break
		}

		startCursor.token = resp.After
		opt.After = resp.After
		startCursor.lastID = ""

		if size >= maxMemorySize {
			f.logger.DebugContext(ctx, "Github audit log size exceeded", "size", size)
			break
		}
		if len(events) > maxElemSize {
			f.logger.DebugContext(ctx, "Github audit log size exceeded", "size", len(events))
			break
		}
	}
	return events, startCursor, true, nil
}

// skipProcessedEntries skips the processed entries in the list of entries.
func skipProcessedEntries(entries []*github.AuditEntry, lastID string) []*github.AuditEntry {
	for i, entry := range entries {
		if entry.GetDocumentID() == lastID {
			return entries[i+1:]
		}
	}
	return entries
}

// auditToProtoStruct converts the given data to a protobuf struct.
// It marshals the data to JSON and then unmarshals it to a protobuf struct.
// TODO(tigrato): find a better way to convert the data to a protobuf struct.
func auditToProtoStruct[T any](data T) (*structpb.Struct, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pb := &structpb.Struct{}
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, pb)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb, nil
}

// pollResults is a struct that holds the results of the poll.
type pollResults struct {
	tokens          []*accessgraphv1alpha.GithubTokenV1
	roleAssignments []*accessgraphv1alpha.GithubRoleAssignmentV1
	roles           []*accessgraphv1alpha.GithubRoleV1
	repos           []*accessgraphv1alpha.GithubRepositoryV1
}

// githubOrgAdminRoles is a map of GitHub organization admin roles.
// These are predefined roles that are used to assign permissions to users in the organization.
var githubOrgAdminRoles = map[string]struct{}{
	"all_repo_admin":    {},
	"all_repo_maintain": {},
	"all_repo_triage":   {},
	"all_repo_write":    {},
	"ci_cd_admin":       {},
	"security_manager":  {},
}

func (f *fetcher) pollGithubState(ctx context.Context) (*pollResults, error) {
	res := &pollResults{}
	tokenOwners := map[string]struct{}{}

	// Fetch tokens for the organization
	if err := f.fetchTokens(ctx, res, tokenOwners); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch organization admins with personal access tokens
	if err := f.fetchAdmins(ctx, res, tokenOwners); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch admin roles and their assignments
	if err := f.fetchAdminRoles(ctx, res); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch repositories and their collaborators
	if err := f.fetchRepositories(ctx, res); err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

func (f *fetcher) fetchTokens(ctx context.Context, res *pollResults, tokenOwners map[string]struct{}) error {
	opts := github.ListFineGrainedPATOptions{}
	for {
		tokens, resp, err := f.client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, f.organizationName, &opts)
		if err != nil {
			return trace.Wrap(err, "failed to list GitHub tokens")
		}

		for _, token := range tokens {
			permissions := f.extractTokenPermissions(token)
			owner := token.GetOwner().GetLogin()
			if owner != "" {
				tokenOwners[owner] = struct{}{}
			}

			res.tokens = append(res.tokens, &accessgraphv1alpha.GithubTokenV1{
				Id:           token.GetID(),
				Name:         token.GetTokenName(),
				Owner:        owner,
				Expires:      timestamppb.New(token.TokenExpiresAt.Time),
				Permissions:  permissions,
				Organization: f.organizationName,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return nil
}

func (f *fetcher) fetchAdmins(ctx context.Context, res *pollResults, tokenOwners map[string]struct{}) error {
	opts := github.ListMembersOptions{PublicOnly: false, Role: "admin"}
	for {
		admins, resp, err := f.client.Organizations.ListMembers(ctx, f.organizationName, &opts)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, admin := range admins {
			if _, hasToken := tokenOwners[admin.GetLogin()]; hasToken {
				res.roleAssignments = append(res.roleAssignments, &accessgraphv1alpha.GithubRoleAssignmentV1{
					Owner:        true,
					User:         admin.GetLogin(),
					Organization: f.organizationName,
				})
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return nil
}

func (f *fetcher) fetchAdminRoles(ctx context.Context, res *pollResults) error {
	roles, _, err := f.client.Organizations.ListRoles(ctx, f.organizationName)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, role := range roles.CustomRepoRoles {
		if _, isAdmin := githubOrgAdminRoles[role.GetName()]; !isAdmin {
			continue
		}

		res.roles = append(res.roles, &accessgraphv1alpha.GithubRoleV1{
			RoleId:       role.GetID(),
			Name:         role.GetName(),
			Organization: f.organizationName,
		})

		users, _, err := f.client.Organizations.ListUsersAssignedToOrgRole(ctx, f.organizationName, role.GetID(), &github.ListOptions{})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, user := range users {
			res.roleAssignments = append(res.roleAssignments, &accessgraphv1alpha.GithubRoleAssignmentV1{
				RoleId:       role.GetID(),
				User:         user.GetLogin(),
				Organization: f.organizationName,
			})
		}
	}
	return nil
}

func (f *fetcher) fetchRepositories(ctx context.Context, res *pollResults) error {
	opts := github.RepositoryListByOrgOptions{}
	for {
		repos, resp, err := f.client.Repositories.ListByOrg(ctx, f.organizationName, &opts)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, repo := range repos {
			if repo.GetOwner() == nil {
				f.logger.ErrorContext(ctx, "GitHub repo is missing owner", "repo", repo.GetName())
				continue
			}

			collaborators, _, err := f.client.Repositories.ListCollaborators(ctx, f.organizationName, repo.GetName(), &github.ListCollaboratorsOptions{Affiliation: "direct"})
			if err != nil {
				return trace.Wrap(err)
			}

			var collabLogins []string
			for _, collab := range collaborators {
				collabLogins = append(collabLogins, collab.GetLogin())
			}

			res.repos = append(res.repos, &accessgraphv1alpha.GithubRepositoryV1{
				Name:          repo.GetName(),
				Collaborators: collabLogins,
				Organization:  f.organizationName,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return nil
}

func (f *fetcher) extractTokenPermissions(token *github.PersonalAccessToken) []*accessgraphv1alpha.GithubTokenV1Permission {
	var permissions []*accessgraphv1alpha.GithubTokenV1Permission
	for domain, perms := range map[string]map[string]string{
		"org":   token.Permissions.Org,
		"repo":  token.Permissions.Repo,
		"other": token.Permissions.Other,
	} {
		for obj, verb := range perms {
			permissions = append(permissions, &accessgraphv1alpha.GithubTokenV1Permission{
				Domain: domain,
				Object: obj,
				Verb:   verb,
			})
		}
	}
	return permissions
}

// testRequiredPermissions checks if the fetcher has the required permissions
// to function correctly. It attempts to list members of the organization,
// roles, repositories, and audit logs. If any of these operations fail due to
// insufficient permissions, it returns an error indicating the missing permission.
// This is used to ensure that the fetcher can operate without running into
// permission issues during its normal operation.
func (f *fetcher) testRequiredPermissions(ctx context.Context) error {
	// Test if the client can list members of the organization.
	// This is a required permission for the fetcher to work.
	testCalls := []struct {
		requiredPermission string
		f                  func(context.Context) error
	}{
		{
			requiredPermission: "Organization Personal Access Tokens",
			f: func(ctx context.Context) error {
				_, _, err := f.client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, f.organizationName, &github.ListFineGrainedPATOptions{})
				return trace.Wrap(err, "failed to list GitHub tokens")
			},
		},
		{
			requiredPermission: "Organization members",
			f: func(ctx context.Context) error {
				_, _, err := f.client.Organizations.ListMembers(ctx, f.organizationName, &github.ListMembersOptions{PublicOnly: false})
				return trace.Wrap(err, "failed to list GitHub organization members")
			},
		},
		{
			requiredPermission: "Organization Roles",
			f: func(ctx context.Context) error {
				_, _, err := f.client.Organizations.ListRoles(ctx, f.organizationName)
				return trace.Wrap(err, "failed to list GitHub organization roles")
			},
		},
		{
			requiredPermission: "Organization Repositories",
			f: func(ctx context.Context) error {
				_, _, err := f.client.Repositories.ListByOrg(ctx, f.organizationName, &github.RepositoryListByOrgOptions{})
				return trace.Wrap(err, "failed to list GitHub organization repositories")
			},
		},
		{
			requiredPermission: "Organization Audit Logs",
			f: func(ctx context.Context) error {
				opt := github.GetAuditLogOptions{
					Phrase: toPtr(
						fmt.Sprintf("created:%s..%s",
							time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339),
							f.clock.Now().UTC().Format(time.RFC3339),
						),
					),
					ListCursorOptions: github.ListCursorOptions{
						PerPage: 1,
					},
				}
				// Use the appropriate function to get the audit log based on whether
				// the client is for an enterprise installation or not.
				getFunc := f.client.Enterprise.GetAuditLog
				if !f.enterprise {
					getFunc = f.client.Organizations.GetAuditLog
				}

				_, _, err := getFunc(ctx, f.organizationName, &opt)
				return trace.Wrap(err, "failed to poll GitHub audit logs")
			},
		},
	}

	var githubErr *github.ErrorResponse
	for _, test := range testCalls {
		if err := test.f(ctx); err != nil && errors.As(err, &githubErr) &&
			githubErr.Response != nil &&
			githubErr.Response.StatusCode == http.StatusNotFound {
			// If the error is a GitHub not found error, it means the user does not have
			// the required permission to perform the operation.
			return trace.AccessDenied(
				"The GitHub application is missing the required read permission for %s. "+
					"To resolve this, update the application's permissions to allow access to "+
					"the %s organization.",
				test.requiredPermission,
				f.organizationName,
			)
		} else if err != nil {
			return trace.Wrap(err, "failed to test required permissions for fetcher")
		} else {
			f.logger.DebugContext(ctx, "Required permission test passed", "permission", test.requiredPermission)
		}
	}

	return nil
}

func toPtr[T any](s T) *T {
	return &s
}
