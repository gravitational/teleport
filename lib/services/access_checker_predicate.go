package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type accessCheckerPredicate struct {
	info         *AccessInfo
	localCluster string
}

// HasRole checks if the checker includes the role
func (a *accessCheckerPredicate) HasRole(role string) bool {
	fmt.Println("-------------------- HasRole")
	return false
}

// RoleNames returns a list of role names
func (a *accessCheckerPredicate) RoleNames() []string {
	isDebugEnabled, debugf := rbacDebugLogger()
	if isDebugEnabled {
		debugf("RoleNames: %q", a.info.Roles)
	}
	return a.info.Roles
}

// Roles returns the list underlying roles this AccessChecker is based on.
func (a *accessCheckerPredicate) Roles() []types.Role {
	fmt.Println("-------------------- Roles")
	return []types.Role{}
}

// CheckAccess checks access to the specified resource.
func (a *accessCheckerPredicate) CheckAccess(r AccessCheckable, state AccessState, matchers ...RoleMatcher) error {
	fmt.Println("-------------------- CheckAccess")
	fmt.Println("%v", r)
	fmt.Println("%v", state)
	fmt.Println("%v", matchers)
	fmt.Println("%v", a.info.Roles)

	roles := make([]string, len(a.info.Roles))
	for n, role := range a.RoleNames() {
		roles[n] = "source=" + role
	}

	resp, err := http.Get("http://127.0.0.1:5000/authorize?" + strings.Join(roles, "&") + "&target=" + r.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}

	if string(body) == "True" {
		return nil
	}

	return trace.AccessDenied("No access")
}

// CheckAccessToRemoteCluster checks access to remote cluster
func (a *accessCheckerPredicate) CheckAccessToRemoteCluster(cluster types.RemoteCluster) error {
	fmt.Println("-------------------- CheckAccessToRemoteCluster")
	return nil
}

// CheckAccessToRule checks access to a rule within a namespace.
func (a *accessCheckerPredicate) CheckAccessToRule(context RuleContext, namespace string, rule string, verb string, silent bool) error {
	fmt.Println("-------------------- CheckAccessRule")
	return nil
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (a *accessCheckerPredicate) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	fmt.Println("-------------------- CheckLoginDuration")
	return []string{}, nil
}

// CheckKubeGroupsAndUsers check if role can login into kubernetes
// and returns two lists of combined allowed groups and users
func (a *accessCheckerPredicate) CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) (groups []string, users []string, err error) {
	return []string{}, []string{}, nil
}

// CheckAWSRoleARNs returns a list of AWS role ARNs role is allowed to assume.
func (a *accessCheckerPredicate) CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error) {
	return []string{}, nil
}

// CheckAzureIdentities returns a list of Azure identities the user is allowed to assume.
func (a *accessCheckerPredicate) CheckAzureIdentities(ttl time.Duration, overrideTTL bool) ([]string, error) {
	return []string{}, nil
}

// CheckGCPServiceAccounts returns a list of GCP service accounts the user is allowed to assume.
func (a *accessCheckerPredicate) CheckGCPServiceAccounts(ttl time.Duration, overrideTTL bool) ([]string, error) {
	return []string{}, nil
}

// CheckAccessToSAMLIdP checks access to the SAML IdP.
//
//nolint:revive // Because we want this to be IdP.
func (a *accessCheckerPredicate) CheckAccessToSAMLIdP(types.AuthPreference) error {
	return nil
}

// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
// for this role set, otherwise it returns ttl unchanged
func (a *accessCheckerPredicate) AdjustSessionTTL(ttl time.Duration) time.Duration {
	fmt.Println("-------------------- AdjustSessionTTL: 1 hour")
	return time.Hour
}

// AdjustClientIdleTimeout adjusts requested idle timeout
// to the lowest max allowed timeout, the most restrictive
// option will be picked
func (a *accessCheckerPredicate) AdjustClientIdleTimeout(ttl time.Duration) time.Duration {
	fmt.Println("-------------------- AdjustClientIdleTimeout")
	return 0
}

// AdjustDisconnectExpiredCert adjusts the value based on the role set
// the most restrictive option will be picked
func (a *accessCheckerPredicate) AdjustDisconnectExpiredCert(disconnect bool) bool {
	return false
}

// CheckAgentForward checks if the role can request agent forward for this
// user.
func (a *accessCheckerPredicate) CheckAgentForward(login string) error {
	return nil
}

// CanForwardAgents returns true if this role set offers capability to forward
// agents.
func (a *accessCheckerPredicate) CanForwardAgents() bool {
	return false
}

// CanPortForward returns true if this RoleSet can forward ports.
func (a *accessCheckerPredicate) CanPortForward() bool {
	return false
}

// DesktopClipboard returns true if the role set has enabled shared
// clipboard for desktop sessions. Clipboard sharing is disabled if
// one or more of the roles in the set has disabled it.
func (a *accessCheckerPredicate) DesktopClipboard() bool {
	return false
}

// RecordDesktopSession returns true if a role in the role set has enabled
// desktop session recoring.
func (a *accessCheckerPredicate) RecordDesktopSession() bool {
	return false
}

// DesktopDirectorySharing returns true if the role set has directory sharing
// enabled. This setting is enabled if one or more of the roles in the set has
// enabled it.
func (a *accessCheckerPredicate) DesktopDirectorySharing() bool {
	return false
}

// MaybeCanReviewRequests attempts to guess if this RoleSet belongs
// to a user who should be submitting access reviews. Because not all rolesets
// are derived from statically assigned roles, this may return false positives.
func (a *accessCheckerPredicate) MaybeCanReviewRequests() bool {
	return false
}

// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
func (a *accessCheckerPredicate) PermitX11Forwarding() bool {
	return false
}

// CanCopyFiles returns true if the role set has enabled remote file
// operations via SCP or SFTP. Remote file operations are disabled if
// one or more of the roles in the set has disabled it.
func (a *accessCheckerPredicate) CanCopyFiles() bool {
	return false
}

// CertificateFormat returns the most permissive certificate format in a
// RoleSet.
func (a *accessCheckerPredicate) CertificateFormat() string {
	return ""
}

// EnhancedRecordingSet returns a set of events that will be recorded
// for enhanced session recording.
func (a *accessCheckerPredicate) EnhancedRecordingSet() map[string]bool {
	return map[string]bool{}
}

// CheckDatabaseNamesAndUsers returns database names and users this role
// is allowed to use.
func (a *accessCheckerPredicate) CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) (names []string, users []string, err error) {
	return []string{}, []string{}, nil
}

// CheckDatabaseRoles returns whether a user should be auto-created in the
// database and a list of database roles to assign.
func (a *accessCheckerPredicate) CheckDatabaseRoles(types.Database) (create bool, roles []string, err error) {
	return false, []string{}, nil
}

// CheckImpersonate checks whether current user is allowed to impersonate
// users and roles
func (a *accessCheckerPredicate) CheckImpersonate(currentUser, impersonateUser types.User, impersonateRoles []types.Role) error {
	return nil
}

// CheckImpersonateRoles checks whether the current user is allowed to
// perform roles-only impersonation.
func (a *accessCheckerPredicate) CheckImpersonateRoles(currentUser types.User, impersonateRoles []types.Role) error {
	return nil
}

// CanImpersonateSomeone returns true if this checker has any impersonation rules
func (a *accessCheckerPredicate) CanImpersonateSomeone() bool {
	return false
}

// LockingMode returns the locking mode to apply with this checker.
func (a *accessCheckerPredicate) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	return constants.LockingModeStrict
}

// ExtractConditionForIdentifier returns a restrictive filter expression
// for list queries based on the rules' `where` conditions.
func (a *accessCheckerPredicate) ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error) {
	return nil, nil
}

// CertificateExtensions returns the list of extensions for each role in the RoleSet
func (a *accessCheckerPredicate) CertificateExtensions() []*types.CertExtension {
	fmt.Printf("-------------------- CertExtensions %v %v\n", a.info.Traits, a.info.Roles)

	// In our case roles are empty
	var certRoles types.CertRoles = types.CertRoles{Roles: []string{}}
	bytes, err := utils.FastMarshal(certRoles)
	if err != nil {
		fmt.Printf("Failed to marshal empty cert roles: %v\n", trace.BadParameter(err.Error()))
	}

	// Traits must be the same
	traits_json, err := json.Marshal(a.info.Traits)
	if err != nil {
		fmt.Printf("Failed to marshal user traits: %v\n", err)
	}

	roles := types.CertExtension{Name: teleport.CertExtensionTeleportRoles, Value: string(bytes)}
	traits := types.CertExtension{Name: teleport.CertExtensionTeleportTraits, Value: string(traits_json)}

	return []*types.CertExtension{&roles, &traits}
}

// GetAllowedSearchAsRoles returns all of the allowed SearchAsRoles.
func (a *accessCheckerPredicate) GetAllowedSearchAsRoles() []string {
	return []string{}
}

// GetAllowedPreviewAsRoles returns all of the allowed PreviewAsRoles.
func (a *accessCheckerPredicate) GetAllowedPreviewAsRoles() []string {
	return []string{}
}

// MaxConnections returns the maximum number of concurrent ssh connections
// allowed.  If MaxConnections is zero then no maximum was defined and the
// number of concurrent connections is unconstrained.
func (a *accessCheckerPredicate) MaxConnections() int64 {
	return 99
}

// MaxSessions returns the maximum number of concurrent ssh sessions per
// connection. If MaxSessions is zero then no maximum was defined and the
// number of sessions is unconstrained.
func (a *accessCheckerPredicate) MaxSessions() int64 {
	return 99
}

// SessionPolicySets returns the list of SessionPolicySets for all roles.
func (a *accessCheckerPredicate) SessionPolicySets() []*types.SessionTrackerPolicySet {
	return []*types.SessionTrackerPolicySet{}
}

// GetAllLogins returns all valid unix logins for the AccessChecker.
func (a *accessCheckerPredicate) GetAllLogins() []string {
	fmt.Println("-------------------- GetAllLogins")
	return []string{}
}

// GetAllowedResourceIDs returns the list of allowed resources the identity for
// the AccessChecker is allowed to access. An empty or nil list indicates that
// there are no resource-specific restrictions.
func (a *accessCheckerPredicate) GetAllowedResourceIDs() []types.ResourceID {
	return []types.ResourceID{}
}

// SessionRecordingMode returns the recording mode for a specific service.
func (a *accessCheckerPredicate) SessionRecordingMode(service constants.SessionRecordingService) constants.SessionRecordingMode {
	return constants.SessionRecordingModeStrict
}

// HostUsers returns host user information matching a server or nil if
// a role disallows host user creation
func (a *accessCheckerPredicate) HostUsers(types.Server) (*HostUsersInfo, error) {
	fmt.Println("-------------------- HostUsers")
	return nil, nil
}

// DesktopGroups returns the desktop groups a user is allowed to create or an access denied error if a role disallows desktop user creation
func (a *accessCheckerPredicate) DesktopGroups(types.WindowsDesktop) ([]string, error) {
	return []string{}, nil
}

// PinSourceIP forces the same client IP for certificate generation and SSH usage
func (a *accessCheckerPredicate) PinSourceIP() bool {
	return false
}

// GetAccessState returns the AccessState for the user given their roles, the
// cluster auth preference, and whether MFA and the user's device were
// verified.
func (a *accessCheckerPredicate) GetAccessState(authPref types.AuthPreference) AccessState {
	return AccessState{}
}

// PrivateKeyPolicy returns the enforced private key policy for this role set,
// or the provided defaultPolicy - whichever is stricter.
func (a *accessCheckerPredicate) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) keys.PrivateKeyPolicy {
	return keys.PrivateKeyPolicyNone
}

// GetKubeResources returns the allowed and denied Kubernetes Resources configured
// for a user.
func (a *accessCheckerPredicate) GetKubeResources(cluster types.KubeCluster) (allowed, denied []types.KubernetesResource) {
	return []types.KubernetesResource{}, []types.KubernetesResource{}
}
