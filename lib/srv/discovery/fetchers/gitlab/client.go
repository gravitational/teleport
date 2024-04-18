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
	"github.com/gravitational/trace"
	gitlab "github.com/xanzy/go-gitlab"
)

type gitlabClient struct {
	client *gitlab.Client
}

func newClient(url, token string) (gitlabClient, error) {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(url))
	if err != nil {
		return gitlabClient{}, trace.Wrap(err)
	}
	return gitlabClient{client: client}, nil
}

const (
	maxPerPage = 100
)

// getGroups returns a list of Gitlab groups
// it uses the Gitlab API to fetch the groups
// accross all pages.
func (g gitlabClient) getGroups() ([]*gitlab.Group, error) {
	opt := &gitlab.ListGroupsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxPerPage,
			Page:    1,
		},
	}
	var groups []*gitlab.Group
	for {
		out, rsp, err := g.client.Groups.ListGroups(opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		groups = append(groups, out...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage

	}
	return groups, nil
}

// getProjects returns a list of Gitlab projects
// it uses the Gitlab API to fetch the projects
// accross all pages.
func (g gitlabClient) getProjects() ([]*gitlab.Project, error) {
	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxPerPage,
			Page:    1,
		},
		Membership: gitlab.Ptr(true),
	}
	var projects []*gitlab.Project
	for {
		out, rsp, err := g.client.Projects.ListProjects(opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		projects = append(projects, out...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage

	}
	return projects, nil
}

// getProjectMembers returns a list of Gitlab project members
// it uses the Gitlab API to fetch the project members
// accross all pages.
func (g gitlabClient) getProjectMembers(projectID int) ([]*gitlab.ProjectMember, error) {
	opt := &gitlab.ListProjectMembersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxPerPage,
			Page:    1,
		},
	}
	var members []*gitlab.ProjectMember
	for {
		out, rsp, err := g.client.ProjectMembers.ListProjectMembers(projectID, opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		members = append(members, out...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage

	}
	return members, nil
}

// getGroupMembers returns a list of Gitlab group members
// it uses the Gitlab API to fetch the group members
// accross all pages.
func (g gitlabClient) getGroupMembers(groupID int) ([]*gitlab.GroupMember, error) {
	opt := &gitlab.ListGroupMembersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxPerPage,
			Page:    1,
		},
	}
	var members []*gitlab.GroupMember
	for {
		out, rsp, err := g.client.Groups.ListGroupMembers(groupID, opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		members = append(members, out...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage

	}
	return members, nil
}

// getUsers returns a list of Gitlab users
// it uses the Gitlab API to fetch the users
// accross all pages.
func (g gitlabClient) getUsers() ([]*gitlab.User, error) {
	opt := &gitlab.ListUsersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: maxPerPage,
			Page:    1,
		},
	}
	var users []*gitlab.User
	for {
		out, rsp, err := g.client.Users.ListUsers(opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		users = append(users, out...)
		if rsp.NextPage == 0 {
			break
		}
		opt.Page = rsp.NextPage

	}
	return users, nil
}
