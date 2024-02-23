package gitlab

import (
	"context"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/plouc/go-gitlab-client/gitlab"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type GitlabFetcher struct {
	client *gitlab.Gitlab
}

func New(token string) *GitlabFetcher {
	client := gitlab.NewGitlab("https://gitlab.com/", "api/v4", token)
	return &GitlabFetcher{
		client: client,
	}
}

type Resources struct {
	GroupMembers   []*accessgraphv1alpha.GitlabGroupMember
	Groups         []*accessgraphv1alpha.GitlabGroup
	Projects       []*accessgraphv1alpha.GitlabProject
	ProjectMembers []*accessgraphv1alpha.GitlabProjectMember
}

func (g *GitlabFetcher) Poll(ctx context.Context) (*Resources, error) {
	// get groups
	groups, groupMembers, err := g.getGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	projects, projectMembers, err := g.getProjects(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Resources{
		Groups:         groups,
		GroupMembers:   groupMembers,
		Projects:       projects,
		ProjectMembers: projectMembers,
	}, nil
}

func (g *GitlabFetcher) getProjects(ctx context.Context) (
	[]*accessgraphv1alpha.GitlabProject,
	[]*accessgraphv1alpha.GitlabProjectMember,
	error,
) {
	projects, _, err := g.client.Projects(&gitlab.ProjectsOptions{Membership: true})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var out []*accessgraphv1alpha.GitlabProject
	var outMembers []*accessgraphv1alpha.GitlabProjectMember
	for _, project := range projects.Items {
		prj := &accessgraphv1alpha.GitlabProject{
			Name: project.Name,
			Path: project.PathWithNamespace,
		}
		out = append(out, prj)

		members, _, err := g.client.ProjectMembers(strconv.Itoa(project.Id), nil)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members.Items {
			outMembers = append(outMembers, &accessgraphv1alpha.GitlabProjectMember{
				Project:     prj,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			})
		}

	}
	return out, outMembers, nil
}

func (g *GitlabFetcher) getGroups(ctx context.Context) (
	[]*accessgraphv1alpha.GitlabGroup,
	[]*accessgraphv1alpha.GitlabGroupMember,
	error,
) {
	groups, _, err := g.client.Groups(&gitlab.GroupsOptions{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var out []*accessgraphv1alpha.GitlabGroup
	var outMembers []*accessgraphv1alpha.GitlabGroupMember
	for _, group := range groups.Items {
		grp := &accessgraphv1alpha.GitlabGroup{
			Name: group.Name,
			Path: group.FullPath,
		}
		out = append(out, grp)

		members, _, err := g.client.GroupMembers(strconv.Itoa(group.Id), nil)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		for _, member := range members.Items {
			outMembers = append(outMembers, &accessgraphv1alpha.GitlabGroupMember{
				Group:       grp,
				Username:    member.Username,
				AccessLevel: accessLevelToStr(member.AccessLevel),
			})
		}

	}
	return out, outMembers, nil
}

func accessLevelToStr(accessLevel int) string {
	switch accessLevel {
	case 10:
		return "Guest"
	case 20:
		return "Reporter"
	case 30:
		return "Developer"
	case 40:
		return "Master"
	case 50:
		return "Owner"
	}
	return "Unknown"
}
