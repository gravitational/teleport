package gitlab

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type gitlabFetcher struct {
	client gitlabClient
}

func newGitlabFetcher(url, token string) (*gitlabFetcher, error) {
	client, err := newClient(url, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check if the credentials are valid
	err = client.testTokenPermissions()
	if isUnauthorized(err) {
		return nil, trace.NewAggregate(ErrGitlabInvalidCredentials, err)
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gitlabFetcher{
		client: client,
	}, nil
}

// resources is a collection of Gitlab resources
type resources struct {
	// GroupMembers is a list of Gitlab group members
	GroupMembers []*accessgraphv1alpha.GitlabGroupMember
	// Groups is a list of Gitlab groups
	Groups []*accessgraphv1alpha.GitlabGroup
	// Projects is a list of Gitlab projects
	Projects []*accessgraphv1alpha.GitlabProject
	// ProjectMembers is a list of Gitlab project members
	ProjectMembers []*accessgraphv1alpha.GitlabProjectMember
	// Users is a list of Gitlab users
	Users []*accessgraphv1alpha.GitlabUser
}

// poll fetches the latest Gitlab resources.
func (g *gitlabFetcher) poll(ctx context.Context) (*resources, error) {
	// get groups
	groups, groupMembers, err := g.getGroups()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	projects, projectMembers, err := g.getProjects()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users, err := g.getUsers(uniqueUsernames(projectMembers, groupMembers))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &resources{
		Groups:         groups,
		GroupMembers:   groupMembers,
		Projects:       projects,
		ProjectMembers: projectMembers,
		Users:          users,
	}, nil
}

func (g *gitlabFetcher) getProjects() (
	[]*accessgraphv1alpha.GitlabProject,
	[]*accessgraphv1alpha.GitlabProjectMember,
	error,
) {
	projects, err := g.client.getProjects()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var out []*accessgraphv1alpha.GitlabProject
	var outMembers []*accessgraphv1alpha.GitlabProjectMember
	for _, project := range projects {
		prj := accessgraphv1alpha.GitlabProject_builder{
			Name:        project.Name,
			Path:        project.PathWithNamespace,
			Description: project.Description,
		}.Build()
		out = append(out, prj)

		members, err := g.client.getProjectMembers(project.ID)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members {
			outMembers = append(outMembers, accessgraphv1alpha.GitlabProjectMember_builder{
				Project:     prj,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			}.Build())
		}

	}
	return out, outMembers, nil
}

func (g *gitlabFetcher) getGroups() (
	[]*accessgraphv1alpha.GitlabGroup,
	[]*accessgraphv1alpha.GitlabGroupMember,
	error,
) {
	groups, err := g.client.getGroups()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var out []*accessgraphv1alpha.GitlabGroup
	var outMembers []*accessgraphv1alpha.GitlabGroupMember
	for _, group := range groups {
		grp := accessgraphv1alpha.GitlabGroup_builder{
			Name:        group.Name,
			Path:        group.FullPath,
			FullName:    group.FullName,
			Description: group.Description,
		}.Build()
		out = append(out, grp)

		members, err := g.client.getGroupMembers(group.ID)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members {
			outMembers = append(outMembers, accessgraphv1alpha.GitlabGroupMember_builder{
				Group:       grp,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			}.Build())
		}

	}
	return out, outMembers, nil
}

func (g *gitlabFetcher) getUsers(usernames []string) (
	[]*accessgraphv1alpha.GitlabUser,
	error,
) {
	users, err := g.client.getUsers(usernames)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []*accessgraphv1alpha.GitlabUser
	for _, user := range users {
		var lastSign *timestamppb.Timestamp
		if user.LastSignInAt != nil {
			lastSign = timestamppb.New(*user.LastSignInAt)
		}
		var identities []*accessgraphv1alpha.GitlabUserIdentity
		for _, identity := range user.Identities {
			identities = append(identities, accessgraphv1alpha.GitlabUserIdentity_builder{
				Provider:  identity.Provider,
				ExternUid: identity.ExternUID,
			}.Build())
		}
		user := accessgraphv1alpha.GitlabUser_builder{
			Username:         user.Username,
			Email:            user.Email,
			Name:             user.Name,
			IsAdmin:          user.IsAdmin,
			Organization:     user.Organization,
			LastSignInAt:     lastSign,
			CanCreateGroup:   user.CanCreateGroup,
			CanCreateProject: user.CanCreateProject,
			TwoFactorEnabled: user.TwoFactorEnabled,
			Identities:       identities,
		}.Build()
		out = append(out, user)

	}
	return out, nil
}

func accessLevelToStr(accessLevel gitlab.AccessLevelValue) accessgraphv1alpha.AccessLevelType {
	switch accessLevel {
	case gitlab.NoPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_NO_PERMISSIONS
	case gitlab.MinimalAccessPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_MINIMAL
	case gitlab.GuestPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_GUEST
	case gitlab.ReporterPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_REPORTER
	case gitlab.DeveloperPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_DEVELOPER
	case gitlab.MaintainerPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_MAINTAINER
	case gitlab.OwnerPermissions:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_OWNER
	default:
		return accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_UNSPECIFIED
	}
}

func isUnauthorized(err error) bool {
	var gitlabErr *gitlab.ErrorResponse
	if errors.As(err, &gitlabErr) {
		if gitlabErr.Response == nil {
			return false
		}
		return gitlabErr.Response.StatusCode == http.StatusUnauthorized ||
			gitlabErr.Response.StatusCode == http.StatusForbidden
	}
	return false
}

// GitlabRawError returns the error message from a Gitlab error or the error message itself.
func GitlabRawError(err error) (str string) {
	const maxErrorLength = 200 * 1024 /* 200KB */

	defer func() {
		if len(str) > maxErrorLength {
			str = str[:maxErrorLength]
		}
	}()
	if err == nil {
		return ""
	}
	var gitlabErr *gitlab.ErrorResponse
	if errors.As(err, &gitlabErr) {
		if gitlabErr.Message == "" {
			return gitlabErr.Error()
		}
		return gitlabErr.Message
	}
	return err.Error()
}

// GitlabHumanReadableError returns a human-readable description of a Gitlab error.
func GitlabHumanReadableError(err error) string {
	if err == nil {
		return ""
	}
	var pollErr *pollError
	if errors.As(err, &pollErr) {
		var gitlabErr *gitlab.ErrorResponse
		if errors.As(err, &gitlabErr) {
			return handleGitlabError(gitlabErr)
		}
		if errors.Is(err, gitlab.ErrNotFound) {
			return "Failed to access GitLab resources. Please check your GitLab URL and try again."
		}

		// the other errors are not GitLab API errors but rather connection or other errors
		return "Failed to access GitLab. Please check that your GitLab instance is reachable from the Teleport Auth server and try again."
	}

	var pushErr *accessGraphPushError
	if errors.As(err, &pushErr) {
		return "Failed to push GitLab resources to Access Graph. Please check the Access Graph configuration and try again."
	}

	return "Failed to access GitLab. Please check the full response for more details."
}

func handleGitlabError(gitlabErr *gitlab.ErrorResponse) string {
	switch gitlabErr.Response.StatusCode {
	case http.StatusUnauthorized:
		/* revoked tokens return 401 */
		if strings.Contains(gitlabErr.Message, "revoked") {
			return "The access token has been revoked. Please create a new access token and try again."
		}
		return "Failed to authenticate with GitLab. Please check your access token and try again."
	case http.StatusForbidden:
		return "Failed to access GitLab resources. Please check your access permissions and try again."
	case http.StatusNotFound:
		return "Failed to access GitLab resources. Please check your GitLab URL and try again."
	case http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusConflict:
		return "Failed to access GitLab. Please check your GitLab instance state and try again."
	case http.StatusTooManyRequests:
		return "The client was rate-limited and couldn't complete the requests. Please ensure the client is not being rate-limited and try again."
	case http.StatusRequestTimeout:
		return "The request to GitLab timed out. Please check your network connection and try again."
	default:
		return "Failed to access GitLab. Please check the full response for more details."
	}
}

func uniqueUsernames(projectMembers []*accessgraphv1alpha.GitlabProjectMember, groupMembers []*accessgraphv1alpha.GitlabGroupMember) []string {
	seen := make(map[string]struct{})
	for _, member := range projectMembers {
		seen[member.GetUsername()] = struct{}{}
	}
	for _, member := range groupMembers {
		seen[member.GetUsername()] = struct{}{}
	}
	var keys []string
	for k := range maps.Keys(seen) {
		keys = append(keys, k)
	}
	return keys
}

// GitlabInstanceConnectionTest tests the connection to a Gitlab instance.
func GitlabInstanceConnectionTest(ctx context.Context, gitlabURL string, token string) error {
	if err := gitlabInstanceReachable(ctx, gitlabURL); err != nil {
		return trace.Wrap(err)
	}
	client, err := newClient(gitlabURL, token)
	if err != nil {
		return trace.Wrap(err)
	}
	// check if the credentials are valid by fetching a single page of projects
	err = client.testTokenPermissions()
	if isUnauthorized(err) {
		return trace.NewAggregate(ErrGitlabInvalidCredentials, err)
	} else if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func gitlabInstanceReachable(ctx context.Context, gitlabURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u, _, err := prepareURL(gitlabURL)
	if err != nil {
		return trace.BadParameter("invalid Gitlab URL: %v", err)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return trace.Wrap(err, "Failed to create request")
	}
	resp, err := client.Do(req)
	if err != nil {
		return trace.ConnectionProblem(err, "Failed to reach Gitlab instance")
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

type pollError struct {
	err error
}

func (p *pollError) Error() string {
	return p.err.Error()
}

func (p *pollError) Unwrap() error {
	return p.err
}

func (p *pollError) Is(err error) bool {
	_, ok := err.(*pollError)
	return ok
}

type accessGraphPushError struct {
	err error
}

func (p *accessGraphPushError) Error() string {
	return p.err.Error()
}

func (p *accessGraphPushError) Unwrap() error {
	return p.err
}

func (p *accessGraphPushError) Is(err error) bool {
	_, ok := err.(*accessGraphPushError)
	return ok
}
