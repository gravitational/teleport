package scim

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

func TestSCIMPatch(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	scimToken := createGenericSCIMPlugin(t, sut)
	baseURL := url.URL{
		Scheme: "https",
		Host:   sut.ProxyAddr,
		Path:   "/v1/webapi/scim/generic",
	}

	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			Token: scimToken,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	authClient := sut.Teleport.Process.GetAuthServer()
	aclClient := authClient.AccessListsInternal
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	t.Run("PATCH Unauthorized", func(t *testing.T) {
		t.Parallel()
		patchOps := map[string]any{
			"schemas":    []string{scimsdk.PatchOpSchema},
			"Operations": "{}",
		}

		httpClientWithWrongToken := &http.Client{
			Transport: &bearerAuthTransport{
				Token: "wrong-token",
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		}

		resp, err := doPatchResource(httpClientWithWrongToken, baseURL.String(), "Group", "group-id-123", patchOps)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("PATCH group with replace displayName", func(t *testing.T) {
		t.Parallel()
		groupName := "patch-group-001"
		acl := common.CreateAccessList(t, sut,
			common.WithName(groupName),
			common.WithTitle("Original Group Name"),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)

		patchedGroup := mustPatchGroup(t, httpClient, baseURL.String(), acl.GetName(), []map[string]any{
			{
				"op":    "replace",
				"path":  "displayName",
				"value": "Updated Group Name",
			},
		})

		require.Equal(t, "Updated Group Name", patchedGroup.DisplayName)
		acl, err := aclClient.GetAccessList(t.Context(), groupName)
		require.NoError(t, err)
		require.Equal(t, "Updated Group Name", acl.Spec.Title)
	})

	t.Run("PATCH group add members", func(t *testing.T) {
		t.Parallel()
		groupName := "patch-group-002"
		common.CreateAccessList(t, sut,
			common.WithName(groupName),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)

		patchedGroup := mustPatchGroup(t, httpClient, baseURL.String(), groupName, []map[string]interface{}{
			{
				"op":   "add",
				"path": "members",
				"value": []map[string]any{
					{"value": "user1"},
					{"value": "user2"},
				},
			},
		})

		require.Len(t, patchedGroup.Members, 2)

		members := getAllMembers(t, aclClient, groupName)
		require.Len(t, members, 2)

		got := []string{members[0].GetName(), members[1].GetName()}
		want := []string{"user1", "user2"}
		require.ElementsMatch(t, want, got)
	})

	t.Run("PATCH group remove members", func(t *testing.T) {
		t.Parallel()
		groupName := "patch-group-003"
		common.CreateAccessList(t, sut,
			common.WithName(groupName),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)

		mustPatchGroup(t, httpClient, baseURL.String(), groupName, []map[string]any{
			{
				"op":   "add",
				"path": "members",
				"value": []map[string]any{
					{"value": "user-to-remove"},
					{"value": "user-to-keep"},
				},
			},
		})

		// Use PATCH to remove one member
		patchedGroup := mustPatchGroup(t, httpClient, baseURL.String(), groupName, []map[string]any{
			{
				"op":   "remove",
				"path": `members[value eq "user-to-remove"]`,
			},
		})
		require.Len(t, patchedGroup.Members, 1)

		members := getAllMembers(t, aclClient, groupName)
		require.Len(t, members, 1)
		require.Equal(t, "user-to-keep", members[0].GetName())
	})

	t.Run("PATCH group replace all members", func(t *testing.T) {
		t.Parallel()
		groupName := "patch-group-004"
		// Create access list
		common.CreateAccessList(t, sut,
			common.WithName(groupName),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)

		mustPatchGroup(t, httpClient, baseURL.String(), groupName, []map[string]any{
			{
				"op":   "add",
				"path": "members",
				"value": []map[string]any{
					{"value": "original-member-1"},
					{"value": "original-member-2"},
				},
			},
		})

		// Use PATCH to replace all members
		patchedGroup := mustPatchGroup(t, httpClient, baseURL.String(), groupName, []map[string]any{
			{
				"op":   "replace",
				"path": "members",
				"value": []map[string]any{
					{"value": "new-member-1"},
					{"value": "new-member-2"},
				},
			},
		})

		require.Len(t, patchedGroup.Members, 2)

		members := getAllMembers(t, aclClient, groupName)
		require.Len(t, members, 2)

		got := []string{members[0].GetName(), members[1].GetName()}
		want := []string{"new-member-1", "new-member-2"}
		require.ElementsMatch(t, want, got)
	})

	t.Run("PATCH userName attribute should fail", func(t *testing.T) {
		t.Parallel()
		scimUser := &scimsdk.User{
			ExternalID: "patch-user-004",
			UserName:   "patch-user-004@example.com",
			Active:     true,
		}
		createdUser, err := scimClient.CreateUser(t.Context(), scimUser)
		require.NoError(t, err)

		// Attempt to change userName (should fail)
		patchUserExpectError(t, httpClient, baseURL.String(), createdUser.ID, []map[string]any{
			{
				"op":    "replace",
				"path":  "userName",
				"value": "new-username@example.com",
			},
		}, http.StatusBadRequest)
	})

	t.Run("PATCH group concurrent add and remove members", func(t *testing.T) {
		t.Parallel()
		groupName := "patch-group-005"
		common.CreateAccessList(t, sut,
			common.WithName(groupName),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)

		numberOfMembers := 20

		membersSet := make(map[string]struct{})
		for i := 1; i <= numberOfMembers; i++ {
			membersSet[fmt.Sprintf("member-%d", i)] = struct{}{}
		}

		var wg errgroup.Group
		wg.SetLimit(5)

		for memberName := range membersSet {
			wg.Go(func() error {
				_, err := patchGroup(httpClient, baseURL.String(), groupName, []map[string]any{{
					"op":    "add",
					"path":  "members",
					"value": []map[string]any{{"value": "member-" + memberName}},
				}})
				return err
			})
		}

		require.NoError(t, wg.Wait())

		allMembers := getAllMembers(t, aclClient, groupName)
		require.Len(t, allMembers, numberOfMembers)

		var wg2 errgroup.Group
		wg.SetLimit(5)
		for k := range membersSet {
			wg2.Go(func() error {
				_, err := patchGroup(httpClient, baseURL.String(), groupName, []map[string]any{{
					"op":   "remove",
					"path": fmt.Sprintf(`members[value eq "member-%s"]`, k),
				}})
				return err
			})
		}
		require.NoError(t, wg2.Wait())

		allMembers = getAllMembers(t, aclClient, groupName)
		require.Empty(t, allMembers)
	})
}

func patchUserExpectError(t *testing.T, httpClient *http.Client, baseURL, userID string, ops []map[string]interface{}, expectedStatus int) {
	t.Helper()
	patchOps := map[string]interface{}{
		"schemas":    []string{scimsdk.PatchOpSchema},
		"Operations": ops,
	}

	resp := mustPatchSCIMResource(t, httpClient, baseURL, "Users", userID, patchOps)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, expectedStatus, resp.StatusCode)
}

func mustPatchGroup(t *testing.T, httpClient *http.Client, baseURL, groupID string, ops []map[string]interface{}) *scimsdk.Group {
	t.Helper()
	patchOps := map[string]any{
		"schemas":    []string{scimsdk.PatchOpSchema},
		"Operations": ops,
	}

	resp := mustPatchSCIMResource(t, httpClient, baseURL, "Groups", groupID, patchOps)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var patchedGroup scimsdk.Group
	err = json.Unmarshal(body, &patchedGroup)
	require.NoError(t, err)
	return &patchedGroup
}

func getAllMembers(t *testing.T, aclClient services.AccessListsInternal, groupName string) []*accesslist.AccessListMember {
	t.Helper()
	fn := func(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
		return aclClient.ListAccessListMembers(ctx, groupName, pageSize, nextToken)
	}
	out, err := stream.Collect(clientutils.Resources(t.Context(), fn))
	require.NoError(t, err)
	return out
}

func mustPatchSCIMResource(t *testing.T, httpClient *http.Client, baseURL, resourceType, resourceID string, operations any) *http.Response {
	t.Helper()
	resp, err := doPatchResource(httpClient, baseURL, resourceType, resourceID, operations)
	require.NoError(t, err)
	return resp
}

func doPatchResource(httpClient *http.Client, baseURL, resourceType, resourceID string, ops any) (*http.Response, error) {
	u := baseURL + "/" + resourceType + "/" + resourceID
	payload, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, u, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/scim+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func patchGroup(httpClient *http.Client, baseURL string, groupID string, ops []map[string]interface{}) (*scimsdk.Group, error) {
	patchOps := map[string]any{
		"schemas":    []string{scimsdk.PatchOpSchema},
		"Operations": ops,
	}

	resp, err := doPatchResource(httpClient, baseURL, "Groups", groupID, patchOps)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var patchedGroup scimsdk.Group
	if err = json.Unmarshal(body, &patchedGroup); err != nil {
		return nil, err
	}
	return &patchedGroup, nil
}
