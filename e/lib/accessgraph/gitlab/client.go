package gitlab

import (
	"net/url"
	"sync"

	"github.com/gravitational/trace"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type gitlabClient struct {
	client            *gitlab.Client
	isPrivateInstance bool
}

func newClient(url, token string) (gitlabClient, error) {
	url, isGitlabcom, err := prepareURL(url)
	if err != nil {
		return gitlabClient{}, trace.Wrap(err)
	}
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(url))
	if err != nil {
		return gitlabClient{}, trace.Wrap(err)
	}
	return gitlabClient{client: client, isPrivateInstance: !isGitlabcom}, nil
}

// getGroups returns a list of Gitlab groups
// it uses the Gitlab API to fetch the groups
// across all pages.
func (g gitlabClient) getGroups() ([]*gitlab.Group, error) {
	opt := &gitlab.ListGroupsOptions{
		ListOptions: getListOptions(),
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

func (g gitlabClient) getMembershipFilter() *bool {
	if !g.isPrivateInstance {
		return gitlab.Ptr(true)
	}
	return nil
}

// getProjects returns a list of Gitlab projects
// it uses the Gitlab API to fetch the projects
// across all pages.
func (g gitlabClient) getProjects() ([]*gitlab.Project, error) {
	opt := &gitlab.ListProjectsOptions{
		ListOptions: getListOptions(),
		Membership:  g.getMembershipFilter(),
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

func (g gitlabClient) testTokenPermissions() error {
	_, _, err := g.client.Projects.ListProjects(
		&gitlab.ListProjectsOptions{
			ListOptions: getListOptions(),
			Membership:  g.getMembershipFilter(),
		})

	return trace.Wrap(err)
}

// getProjectMembers returns a list of Gitlab project members
// it uses the Gitlab API to fetch the project members
// across all pages.
func (g gitlabClient) getProjectMembers(projectID int64) ([]*gitlab.ProjectMember, error) {
	opt := &gitlab.ListProjectMembersOptions{
		ListOptions: getListOptions(),
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
// across all pages.
func (g gitlabClient) getGroupMembers(groupID int64) ([]*gitlab.GroupMember, error) {
	opt := &gitlab.ListGroupMembersOptions{
		ListOptions: getListOptions(),
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
// across all pages.
func (g gitlabClient) getUsers(usernames []string) ([]*gitlab.User, error) {
	// gitlab.com makes all users public and we do not need to
	// get only the subset of interesting users.
	if !g.isPrivateInstance {
		users, err := g.getGetUsersGitlabcom(usernames)
		return users, trace.Wrap(err)
	}

	// if it's a private instance we can get all users
	// using the listUsers API.
	opt := &gitlab.ListUsersOptions{
		ListOptions: getListOptions(),
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

// getGetUsersGitlabcom is the implementation of getUsers for gitlab.com
// because gitlab.com makes all users public so using the listUsers API
// without any filter would return all users registered on gitlab.com
// which is not what we want.
// This function uses the listUsers API to get the users with the usernames
// provided in the input.
func (g gitlabClient) getGetUsersGitlabcom(users []string) ([]*gitlab.User, error) {
	type result struct {
		user *gitlab.User
		err  error
	}
	var (
		resultsChan = make(chan result, len(users))
		wg          sync.WaitGroup
		tokens      = make(chan struct{}, 5)
	)
	for _, user := range users {
		wg.Add(1)
		tokens <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-tokens }()
			opt := &gitlab.ListUsersOptions{
				ListOptions: getListOptions(),
				// get only the user with the username
				Username: gitlab.Ptr(user),
			}
			var user *gitlab.User
			userRsp, _, err := g.client.Users.ListUsers(opt)
			if len(userRsp) > 0 {
				user = userRsp[0]
			}
			resultsChan <- result{user: user, err: err}
		}()
	}
	wg.Wait()
	close(resultsChan)
	var (
		results []*gitlab.User
		errs    []error
	)
	for res := range resultsChan {
		if res.err != nil {
			errs = append(errs, res.err)
		}
		if res.user != nil {
			results = append(results, res.user)
		}
	}

	return results, trace.NewAggregate(errs...)
}

func getListOptions() gitlab.ListOptions {
	const maxPerPage = 100
	// TODO(tigrato): use new gitlab keyset pagination
	// once it's generally available and
	// we do not need to support older versions.
	return gitlab.ListOptions{
		PerPage: maxPerPage,
		Page:    1,
	}
}

func prepareURL(u string) (newURL string, isGitlabcom bool, err error) {
	p, err := url.Parse(u)
	if err != nil {
		return "", false, trace.Wrap(err)
	}
	if p.Scheme == "" {
		p.Scheme = "https"
	}

	// gitlab.com is the default gitlab instance
	// We have a special case for gitlab.com because
	// it makes all users public and we need to
	// get only the subset of users.
	const gitlabcom = "gitlab.com"
	return p.String(), p.Host == gitlabcom, nil
}
