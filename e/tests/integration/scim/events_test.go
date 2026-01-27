package scim

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils/slices"
)

func TestAuditEvents(t *testing.T) {
	t.Parallel()

	logger := slog.With("test", t.Name())
	ctx := t.Context()
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithLogger(logger),
	)
	auth := sut.Teleport.Process.GetAuthServer()

	scimToken := createGenericSCIMPlugin(t, sut)
	goodScimClient := createPluginSCIMClient(t, sut, scimToken, "generic")
	badScimClient := createPluginSCIMClient(t, sut, "not-"+scimToken, "generic")

	t.Run("List", func(t *testing.T) {
		const userCount = 4
		users := make([]types.User, 0, userCount)
		for n := range userCount {
			users = append(users, mustCreateSCIMUser(t, sut, fmt.Sprintf("scim-user-%03d", n)))
		}

		common.CreateAccessList(t, sut,
			common.WithCleanup,
			common.WithName("test-group-001"),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
			common.WithMembers(slices.Map(users, types.User.GetName)...))

		t.Run("Users", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListUsers(ctx)
				require.NoError(t, err)
				eventLog.requireEvent(t, events.SCIMListingEvent,
					withListingMetadata(
						withEventCode(events.SCIMListResourcesSuccessCode)),
					withListingStatus(
						withSuccess(true),
						withNoError),
					withListingCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withFilter(""),
					withResourceCount(userCount))
			})

			t.Run("OnAccessDenied", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := badScimClient.ListUsers(ctx)
				require.Error(t, err)
				eventLog.requireNoEvent(t, events.SCIMListingEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListGroups(ctx)
				require.NoError(t, err)
				eventLog.requireEvent(t, events.SCIMListingEvent,
					withListingMetadata(
						withEventCode(events.SCIMListResourcesSuccessCode)),
					withListingStatus(
						withSuccess(true),
						withNoError),
					withListingCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withFilter(""),
					withResourceCount(1))
			})
		})
	})

	t.Run("FilteredList", func(t *testing.T) {
		const userCount = 4
		users := make([]types.User, 0, userCount)
		for n := range userCount {
			users = append(users, mustCreateSCIMUser(t, sut, fmt.Sprintf("user-%03d", n)))
		}

		common.CreateAccessList(t, sut,
			common.WithCleanup,
			common.WithName("test-group-001"),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
			common.WithMembers(slices.Map(users, types.User.GetName)...))

		t.Run("Users", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListUsers(ctx, scimsdk.WithFilter(`userName eq "user-003"`))
				require.NoError(t, err)
				eventLog.requireEvent(t, events.SCIMListingEvent,
					withListingMetadata(
						withEventCode(events.SCIMListResourcesSuccessCode)),
					withListingStatus(
						withSuccess(true),
						withNoError),
					withListingCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withFilter(`userName eq "user-003"`),
					withResourceCount(1))
			})

			t.Run("OnEmptyResponse", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListUsers(ctx, scimsdk.WithFilter(`userName eq "no-such-user"`))
				require.NoError(t, err)
				eventLog.requireEvent(t, events.SCIMListingEvent,
					withListingMetadata(
						withEventCode(events.SCIMListResourcesSuccessCode)),
					withListingStatus(
						withSuccess(true),
						withNoError),
					withListingCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withFilter(`userName eq "no-such-user"`),
					withResourceCount(0))
			})

			t.Run("OnBadFilter", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListUsers(ctx, scimsdk.WithFilter("i'm a potato"))
				require.Error(t, err)
				eventLog.requireNoEvent(t, events.SCIMListingEvent)
			})

			t.Run("OnAccessDenied", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := badScimClient.GetUserByUserName(ctx, "user-003")
				require.Error(t, err)
				eventLog.requireNoEvent(t, events.SCIMListingEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				eventLog := newLogScope[*apievents.SCIMListingEvent](sut)
				_, err := goodScimClient.ListGroups(ctx, scimsdk.WithFilter(`displayName eq "test-group-001"`))
				require.NoError(t, err)
				eventLog.requireEvent(t, events.SCIMListingEvent,
					withListingMetadata(
						withEventCode(events.SCIMListResourcesSuccessCode)),
					withListingStatus(
						withSuccess(true),
						withNoError),
					withListingCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withFilter(`displayName eq "test-group-001"`),
					withResourceCount(1))
			})
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Users", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				scimUser := newSCIMUser("create-test-user")
				_, err := goodScimClient.CreateUser(ctx, scimUser)
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, auth.DeleteUser(ctx, scimUser.UserName))
				})
				auditLog.requireEvent(t, events.SCIMCreateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceCreateSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withExternalID(scimUser.ExternalID),
					withTeleportID(scimUser.UserName),
					withResponseStatusCode(http.StatusCreated),
					withResponseBody(
						fieldEquals("id", "create-test-user@example.com"),
						fieldEquals("externalId", "create-test-user"),
					),
				)
			})

			t.Run("OnInvalidUsername", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				scimUser := newSCIMUser("Gráinne-O'Malley")
				_, err := goodScimClient.CreateUser(ctx, scimUser)
				require.Error(t, err)
				auditLog.requireEvent(t, events.SCIMCreateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceCreateFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withErrorMatching(`.*special characters are not allowed.*`)),
					withExternalID(scimUser.ExternalID),
					withTeleportID(""),
					withNoResponse,
				)
			})

			t.Run("OnAccessDenied", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				scimUser := newSCIMUser("create-test-user")
				_, err := badScimClient.CreateUser(ctx, scimUser)
				require.Error(t, err)
				auditLog.requireNoEvent(t, events.SCIMCreateEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			// The Generic SCIM implementation can't create Access Lists for
			// groups, and needs an existing Access List to bind to or the create
			// request will fail.
			common.CreateAccessList(t, sut,
				common.WithCleanup,
				common.WithName("test-group-001"),
				common.WithTitle("Test Group #01"),
				common.WithOwners("alice-admin"),
				common.WithAccessListType(accesslist.SCIM),
				common.WithGrants(accesslist.Grants{Roles: []string{"access"}}))

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				scimGroup, err := goodScimClient.CreateGroup(ctx, &scimsdk.Group{
					DisplayName: "Test Group #01",
				})
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMCreateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceCreateSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withExternalID(""), // we don't know what the upstream system calls groups
					withTeleportID(scimGroup.ID),
					withDisplayName(scimGroup.DisplayName),
					withResponseStatusCode(http.StatusCreated),
					withResponseBody(
						fieldNotEmpty("id"),
						fieldEquals("displayName", "Test Group #01"),
						fieldLen("members", 0),
					),
				)
			})

			t.Run("OnNoSuchGroup", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)

				// When I try to create a group that does not have a pre-existing
				// Access List to back it...
				_, err := goodScimClient.CreateGroup(ctx, &scimsdk.Group{
					DisplayName: "No Such Group",
				})
				require.Error(t, err)

				auditLog.requireEvent(t, events.SCIMCreateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceCreateFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withErrorMatching("does not exist")),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withExternalID(""),
					withTeleportID(""),
					withDisplayName(""),
					withNoResponse,
				)
			})
		})
	})

	t.Run("Get", func(t *testing.T) {
		var users []types.User
		for n := range 4 {
			users = append(users, mustCreateSCIMUser(t, sut, fmt.Sprintf("user-%03d", n)))
		}

		accessList := common.CreateAccessList(t, sut,
			common.WithCleanup,
			common.WithName("get-resource-gest-group"),
			common.WithTitle("Test group for fetching SCIM resources"),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
			common.WithMembers(slices.Map(users, types.User.GetName)...))

		t.Run("Users", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.GetUser(ctx, "user-002")
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMGetEvent,
					withResourceMetadata(
						withEventCode(events.SCIMGetResourceSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withExternalID("user-002-external-id"),
					withTeleportID("user-002"),
					withNoResponse)
			})

			t.Run("OnNoSuchResource", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.GetUser(ctx, "no-such-user")
				require.Error(t, err)
				auditLog.requireEvent(t, events.SCIMGetEvent,
					withResourceMetadata(
						withEventCode(events.SCIMGetResourceFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withTeleportID("no-such-user"),
					withExternalID(""),
					withNoResponse)
			})

			t.Run("OnUnauthorized", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := badScimClient.GetUser(ctx, "no-such-user")
				require.Error(t, err)
				auditLog.requireNoEvent(t, events.SCIMGetEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.GetGroup(ctx, accessList.GetName())
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMGetEvent,
					withResourceMetadata(
						withEventCode(events.SCIMGetResourceSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withExternalID(""),
					withTeleportID(accessList.GetName()),
					withDisplayName("Test group for fetching SCIM resources"))
			})
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Users", func(t *testing.T) {
			mustCreateSCIMUser(t, sut, "update-test-user")

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.UpdateUser(ctx, &scimsdk.User{
					ID:         "update-test-user",
					ExternalID: "update-test-user-external-id",
					UserName:   "update-test-user",
					Active:     false,
				})
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMUpdateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceUpdateSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withExternalID("update-test-user-external-id"),
					withTeleportID("update-test-user"),
					withResponseStatusCode(http.StatusOK),
					withResponseBody(
						fieldEquals("id", "update-test-user"),
					),
				)
			})

			t.Run("OnNoSuchResource", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.UpdateUser(ctx, &scimsdk.User{
					ID:         "no-such-user-to-update",
					ExternalID: "no-such-user-to-update-external-id",
					UserName:   "no-such-user-to-update",
					Active:     true,
				})
				require.Error(t, err)
				auditLog.requireEvent(t, events.SCIMUpdateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceUpdateFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withTeleportID("no-such-user-to-update"),
					withExternalID(""),
					withNoResponse)
			})

			t.Run("OnUnauthorized", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := badScimClient.UpdateUser(ctx, &scimsdk.User{
					ID:         "update-test-user",
					ExternalID: "update-test-user-external-id",
					UserName:   "update-test-user",
					Active:     false,
				})
				require.Error(t, err)
				auditLog.requireNoEvent(t, events.SCIMUpdateEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			accessList := common.CreateAccessList(t, sut,
				common.WithCleanup,
				common.WithName("update-resource-test-group"),
				common.WithTitle("Test group for udating SCIM resources"),
				common.WithAccessListType(accesslist.SCIM),
				common.WithOwners("alice-admin"),
				common.WithGrants(accesslist.Grants{Roles: []string{"access"}}))

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				_, err := goodScimClient.UpdateGroup(ctx, &scimsdk.Group{
					ID:          accessList.GetName(),
					DisplayName: "Updated Access List!",
				})
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMUpdateEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceUpdateSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withExternalID(""),
					withResponseStatusCode(http.StatusOK),
					withResponseBody(
						fieldEquals("displayName", "Updated Access List!"),
					))
			})
		})
	})

	t.Run("Patch", func(t *testing.T) {
		httpClient := newBearerClient(scimToken)
		baseURL := "https://" + sut.ProxyAddr + "/v1/webapi/scim/generic"

		t.Run("Users", func(t *testing.T) {
			targetUser := mustCreateSCIMUser(t, sut, "patch-test-user")

			patch := map[string]any{
				"Schemas": []any{scimsdk.PatchOpSchema},
				"operations": []any{
					map[string]any{
						"op":    scimsdk.OpReplace,
						"path":  "nickName",
						"value": "dave",
					},
				},
			}

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)

				resp := mustPatchSCIMResource(t, httpClient, baseURL, "Users", targetUser.GetName(), patch)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusOK, resp.StatusCode)

				auditLog.requireEvent(t, events.SCIMPatchEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourcePatchSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withExternalID("patch-test-user-external-id"),
					withTeleportID("patch-test-user"),
					withRequestBody(withExactly(patch)),
					withResponseStatusCode(http.StatusOK),
					withResponseBody(
						fieldEquals("id", targetUser.GetName()),
					),
				)
			})

			t.Run("OnNoSuchResource", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)

				resp := mustPatchSCIMResource(t, httpClient, baseURL, "Users", "no-such-user-to-patch", patch)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusNotFound, resp.StatusCode)

				auditLog.requireEvent(t, events.SCIMPatchEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourcePatchFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withTeleportID("no-such-user-to-patch"),
					withExternalID(""),
					withRequestBody(withExactly(patch)),
					withNoResponse)
			})

			t.Run("OnUnauthorized", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)

				resp := mustPatchSCIMResource(t, newBearerClient("this-is-a-bad-token"),
					baseURL, "Users", "no-such-user-to-patch", patch)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

				auditLog.requireNoEvent(t, events.SCIMPatchEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			accessList := common.CreateAccessList(t, sut,
				common.WithCleanup,
				common.WithName("patch-resource-test-group"),
				common.WithTitle("Test group for patching SCIM resources"),
				common.WithAccessListType(accesslist.SCIM),
				common.WithOwners("alice-admin"),
				common.WithGrants(accesslist.Grants{Roles: []string{"access"}}))

			patch := map[string]any{
				"Schemas": []any{scimsdk.PatchOpSchema},
				"operations": []any{
					map[string]any{
						"op":    scimsdk.OpAdd,
						"path":  "displayName",
						"value": "Updated Access List display name!",
					},
				},
			}

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)

				resp := mustPatchSCIMResource(t, httpClient, baseURL, "Groups", accessList.GetName(), patch)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusOK, resp.StatusCode)

				auditLog.requireEvent(t, events.SCIMPatchEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourcePatchSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withExternalID(""),
					withTeleportID(accessList.GetName()),
					withDisplayName("Updated Access List display name!"),
					withRequestBody(withExactly(patch)),
					withResponseStatusCode(http.StatusOK),
					withResponseBody(
						fieldNotEmpty("id"),
						fieldEquals("displayName", "Updated Access List display name!"),
					),
				)
			})
		})
	})

	t.Run("DeleteResource", func(t *testing.T) {
		t.Run("Users", func(t *testing.T) {
			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				scimUser := mustCreateSCIMUser(t, sut, "delete-test-user")
				err := goodScimClient.DeleteUser(ctx, scimUser.GetName())
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMDeleteEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceDeleteSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withTeleportID("delete-test-user"),
					withExternalID(""),
					withNoResponse)
			})

			t.Run("OnNoSuchResource", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				err := goodScimClient.DeleteUser(ctx, "no-such-user")
				require.Error(t, err)
				auditLog.requireEvent(t, events.SCIMDeleteEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceDeleteFailureCode)),
					withResourceStatus(
						withSuccess(false),
						withError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Users")),
					withExternalID(""),
					withTeleportID("no-such-user"),
					withNoResponse)
			})

			t.Run("OnUnauthorized", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				err := badScimClient.DeleteUser(ctx, "delete-test-user")
				require.Error(t, err)
				auditLog.requireNoEvent(t, events.SCIMDeleteEvent)
			})
		})

		t.Run("Groups", func(t *testing.T) {
			accessList := common.CreateAccessList(t, sut,
				common.WithCleanup,
				common.WithName("delete-resource-test-group"),
				common.WithTitle("Test group for deleting SCIM resources"),
				common.WithAccessListType(accesslist.SCIM),
				common.WithOwners("alice-admin"),
				common.WithGrants(accesslist.Grants{Roles: []string{"access"}}))

			t.Run("OnSuccess", func(t *testing.T) {
				auditLog := newLogScope[*apievents.SCIMResourceEvent](sut)
				err := goodScimClient.DeleteGroup(ctx, accessList.GetName())
				require.NoError(t, err)
				auditLog.requireEvent(t, events.SCIMDeleteEvent,
					withResourceMetadata(
						withEventCode(events.SCIMResourceDeleteSuccessCode)),
					withResourceStatus(
						withSuccess(true),
						withNoError),
					withResourceCommonData(
						withIntegration("generic"),
						withResourceType("Groups")),
					withTeleportID("delete-resource-test-group"),
					withDisplayName(""),
					withExternalID(""),
					withNoResponse)
			})
		})
	})
}
