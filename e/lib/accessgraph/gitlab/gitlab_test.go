package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestGitlab(t *testing.T) {
	tt := []struct {
		name         string
		publicGitlab bool
	}{
		{
			name:         "public gitlab",
			publicGitlab: true,
		},
		{
			name:         "private gitlab",
			publicGitlab: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			url := newGitlabFakeServer(t, tc.publicGitlab)
			client, err := newGitlabFetcher(url, "token")
			if !assert.NoError(t, err) {
				return
			}
			client.client.isPrivateInstance = !tc.publicGitlab
			res, err := client.poll(context.Background())
			if !assert.NoError(t, err) {
				return
			}
			expectedUsers := []*accessgraphv1alpha.GitlabUser{
				accessgraphv1alpha.GitlabUser_builder{Username: "jack_smith", Name: "Jack Smith"}.Build(),
				accessgraphv1alpha.GitlabUser_builder{Username: "john_smith", Name: "John Smith"}.Build(),
			}
			sort.Slice(res.Users, func(i, j int) bool {
				return res.Users[i].GetUsername() < res.Users[j].GetUsername()
			})
			require.Empty(t,
				cmp.Diff(
					expectedUsers,
					res.Users,
					protocmp.Transform(),
				))
			expectedGroups := []*accessgraphv1alpha.GitlabGroup{
				accessgraphv1alpha.GitlabGroup_builder{
					Name:        "Foobar Group",
					Path:        "foo-bar",
					Description: "An interesting group",
				}.Build(),
			}
			require.Empty(t,
				cmp.Diff(
					expectedGroups,
					res.Groups,
					protocmp.Transform(),
				))
			expectedProjects := []*accessgraphv1alpha.GitlabProject{
				accessgraphv1alpha.GitlabProject_builder{
					Name:        "Diaspora Client",
					Path:        "diaspora/diaspora-client",
					Description: "",
				}.Build(),
			}
			require.Empty(t,
				cmp.Diff(
					expectedProjects,
					res.Projects,
					protocmp.Transform(),
				))
			expectedGroupMembers := []*accessgraphv1alpha.GitlabGroupMember{
				accessgraphv1alpha.GitlabGroupMember_builder{
					Username: "john_smith",
					Group: accessgraphv1alpha.GitlabGroup_builder{
						Name:        "Foobar Group",
						Path:        "foo-bar",
						Description: "An interesting group",
					}.Build(),
					AccessLevel: accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_DEVELOPER,
				}.Build(),
			}
			require.Empty(t,
				cmp.Diff(
					expectedGroupMembers,
					res.GroupMembers,
					protocmp.Transform(),
				))
			expectedProjectMembers := []*accessgraphv1alpha.GitlabProjectMember{
				accessgraphv1alpha.GitlabProjectMember_builder{
					Username: "jack_smith",
					Project: accessgraphv1alpha.GitlabProject_builder{
						Name:        "Diaspora Client",
						Path:        "diaspora/diaspora-client",
						Description: "",
					}.Build(),
					AccessLevel: accessgraphv1alpha.AccessLevelType_ACCESS_LEVEL_TYPE_DEVELOPER,
				}.Build(),
			}
			require.Empty(t,
				cmp.Diff(
					expectedProjectMembers,
					res.ProjectMembers,
					protocmp.Transform(),
				))

		})
	}

}

func newGitlabFakeServer(t *testing.T, publicGitlab bool) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/api/v4/groups", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK)
		const groups = `[
			{
			  "id": 1,
			  "name": "Foobar Group",
			  "full_path": "foo-bar",
			  "description": "An interesting group"
			}
		  ]`
		w.Write([]byte(groups))
	}))
	mux.Handle("/api/v4/projects", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		const projects = `[
			{
			  "id": 4,
			  "description": null,
			  "name": "Diaspora Client",
			  "name_with_namespace": "Diaspora / Diaspora Client",
			  "path": "diaspora-client",
			  "path_with_namespace": "diaspora/diaspora-client",
			  "created_at": "2013-09-30T13:46:02Z"
			}
			]`
		w.Write([]byte(projects))
	}))

	mux.Handle("/api/v4/groups/1/members", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		const groupMembers = `[
			{
			  "id": 1,
			  "username": "john_smith",
			  "name": "John Smith",
			  "state": "active",
			  "access_level": 30,
			  "group_saml_identity": null
			}
			]`
		w.Write([]byte(groupMembers))
	}))

	mux.Handle("/api/v4/projects/4/members", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		const projectMembers = `[
			{
			  "id": 2,
			  "username": "jack_smith",
			  "name": "jack smith",
			  "state": "active",
			  "access_level": 30,
			  "group_saml_identity": null
			}
			]`
		w.Write([]byte(projectMembers))
	}))

	mux.Handle("/api/v4/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		const john = `{
			"id": 1,
			"username": "john_smith",
			"name": "John Smith",
			"state": "active"
		  }`
		const jack = `{
			"id": 2,
			"username": "jack_smith",
			"name": "Jack Smith",
			"state": "active"
		  }`
		results := []string{john, jack}
		if r.URL.Query().Get("username") == "john_smith" {
			results = []string{john}
		} else if r.URL.Query().Get("username") == "jack_smith" {
			results = []string{jack}
		} else if !publicGitlab && r.URL.Query().Has("username") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		users := `[` + strings.Join(results, ",") + `]`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(users))
	}))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}
