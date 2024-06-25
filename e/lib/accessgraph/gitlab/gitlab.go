package gitlab

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	gitlab "github.com/xanzy/go-gitlab"
	"golang.org/x/exp/maps"
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
	_, err = client.getProjects()
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
		prj := &accessgraphv1alpha.GitlabProject{
			Name:        project.Name,
			Path:        project.PathWithNamespace,
			Description: project.Description,
		}
		out = append(out, prj)

		members, err := g.client.getProjectMembers(project.ID)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members {
			outMembers = append(outMembers, &accessgraphv1alpha.GitlabProjectMember{
				Project:     prj,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			})
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
		grp := &accessgraphv1alpha.GitlabGroup{
			Name:        group.Name,
			Path:        group.FullPath,
			FullName:    group.FullName,
			Description: group.Description,
		}
		out = append(out, grp)

		members, err := g.client.getGroupMembers(group.ID)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members {
			outMembers = append(outMembers, &accessgraphv1alpha.GitlabGroupMember{
				Group:       grp,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			})
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
			identities = append(identities, &accessgraphv1alpha.GitlabUserIdentity{
				Provider:  identity.Provider,
				ExternUid: identity.ExternUID,
			})
		}
		user := &accessgraphv1alpha.GitlabUser{
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
		}
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

// GitlabMessageOrError returns the error message from a Gitlab error or the error message itself.
func GitlabMessageOrError(err error) string {
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

func uniqueUsernames(projectMembers []*accessgraphv1alpha.GitlabProjectMember, groupMembers []*accessgraphv1alpha.GitlabGroupMember) []string {
	seen := make(map[string]struct{})
	for _, member := range projectMembers {
		seen[member.Username] = struct{}{}
	}
	for _, member := range groupMembers {
		seen[member.Username] = struct{}{}
	}
	return maps.Keys(seen)
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
	_, err = client.getProjects()
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
