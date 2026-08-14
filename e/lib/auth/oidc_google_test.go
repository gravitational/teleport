package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

func TestOIDCGoogle(t *testing.T) {
	t.Parallel()

	directGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
	}

	// group2@sub.foo.example is in group3@bar.example and group3@bar.example is in group4@bar.example
	strictDirectGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example"},
	}
	directIndirectGroups := map[string][]string{
		"alice@foo.example":  {"group3@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group3@bar.example"},
	}
	indirectGroups := map[string][]string{
		"alice@foo.example":  {"group4@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group4@bar.example"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/directory/v1/groups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)

		email := r.URL.Query().Get("userKey")
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		domain := r.URL.Query().Get("domain")

		resp := &directory.Groups{}
		for _, groupEmail := range directGroups[email] {
			if domain == "" || strings.HasSuffix(groupEmail, "@"+domain) {
				resp.Groups = append(resp.Groups, &directory.Group{Email: groupEmail})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/v1/groups/-/memberships:searchTransitiveGroups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		q := r.URL.Query().Get("query")

		// hacky solution but the query parameter of searchTransitiveGroups is also pretty hacky
		prefix := "member_key_id == '"
		suffix := "' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels"
		require.True(t, strings.HasPrefix(q, prefix))
		require.True(t, strings.HasSuffix(q, suffix))
		email := strings.TrimSuffix(strings.TrimPrefix(q, prefix), suffix)
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		resp := &cloudidentity.SearchTransitiveGroupsResponse{}

		for relationType, groupEmails := range map[string][]string{
			"DIRECT":              strictDirectGroups[email],
			"DIRECT_AND_INDIRECT": directIndirectGroups[email],
			"INDIRECT":            indirectGroups[email],
		} {
			for _, groupEmail := range groupEmails {
				resp.Memberships = append(resp.Memberships, &cloudidentity.GroupRelation{
					GroupKey: &cloudidentity.EntityKey{
						Id: groupEmail,
					},
					Labels: map[string]string{
						"cloudidentity.googleapis.com/groups.discussion_forum": "",
					},
					RelationType: relationType,
				})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	testOptions := []option.ClientOption{option.WithEndpoint(ts.URL), option.WithoutAuthentication()}

	ctx := context.Background()

	for _, testCase := range []struct {
		email, domain                string
		transitive, direct, filtered []string
	}{
		{
			"alice@foo.example", "foo.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group1@foo.example"},
		},
		{
			"bob@foo.example", "foo.example",
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
		},
		{
			"carlos@bar.example", "bar.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group3@bar.example"},
		},
	} {
		// transitive groups
		groups, err := groupsFromGoogleCloudIdentity(ctx, testCase.email, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.transitive, groups)

		// direct groups, unfiltered
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, "", testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.direct, groups)

		// direct groups, filtered by domain
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, testCase.domain, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.filtered, groups)
	}
}
