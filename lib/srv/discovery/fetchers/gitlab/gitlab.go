/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package gitlab

import (
	"context"

	"github.com/gravitational/trace"
	gitlab "github.com/xanzy/go-gitlab"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type GitlabFetcher struct {
	client gitlabClient
}

func New(url, token string) (*GitlabFetcher, error) {
	client, err := newClient(url, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &GitlabFetcher{
		client: client,
	}, nil
}

// Resources is a collection of Gitlab resources
type Resources struct {
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

// Poll fetches the latest Gitlab resources.
func (g *GitlabFetcher) Poll(ctx context.Context) (*Resources, error) {
	// get groups
	groups, groupMembers, err := g.getGroups()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	projects, projectMembers, err := g.getProjects()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users, err := g.getUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Resources{
		Groups:         groups,
		GroupMembers:   groupMembers,
		Projects:       projects,
		ProjectMembers: projectMembers,
		Users:          users,
	}, nil
}

func (g *GitlabFetcher) getProjects() (
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

func (g *GitlabFetcher) getGroups() (
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

func (g *GitlabFetcher) getUsers() (
	[]*accessgraphv1alpha.GitlabUser,
	error,
) {

	users, err := g.client.getUsers()
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
