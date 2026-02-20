// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joining

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"slices"
	"time"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/scopes"
)

var rolesSupportingScopes = types.SystemRoles{
	types.RoleNode,
}

// TokenUsageMode represents the possible usage modes of a scoped token.
type TokenUsageMode string

const (
	// TokenUsageModeSingle denotes a token that can only provision a single resource.
	TokenUsageModeSingle TokenUsageMode = "single_use"
	// TokenUsageModeUnlimited denotes a token that can provision any number of resources.
	TokenUsageModeUnlimited = "unlimited"
)

func validateJoinMethod(token *joiningv1.ScopedToken) error {

	switch types.JoinMethod(token.GetSpec().GetJoinMethod()) {
	case types.JoinMethodToken:
		if token.GetStatus().GetSecret() == "" {
			return trace.BadParameter("secret value must be defined for a scoped token when using the token join method")
		}
	case types.JoinMethodEC2, types.JoinMethodIAM:
		if len(token.GetSpec().GetAws().GetAllow()) == 0 {
			return trace.BadParameter("aws configuration must be defined for a scoped token when using the ec2 or iam join methods")
		}
	case types.JoinMethodGCP:
		if len(token.GetSpec().GetGcp().GetAllow()) == 0 {
			return trace.BadParameter("gcp configuration must be defined for a scoped token when using the gcp join method")
		}
	case types.JoinMethodAzure:
		if len(token.GetSpec().GetAzure().GetAllow()) == 0 {
			return trace.BadParameter("azure configuration must be defined for a scoped token when using the azure join method")
		}
	case types.JoinMethodAzureDevops:
		if len(token.GetSpec().GetAzureDevops().GetAllow()) == 0 {
			return trace.BadParameter("azure_devops configuration must be defined for a scoped token when using the azure_devops join method")
		}
	case types.JoinMethodOracle:
		if len(token.GetSpec().GetOracle().GetAllow()) == 0 {
			return trace.BadParameter("oracle configuration must be defined for a scoped token when using the oracle join method")
		}
	default:
		return trace.BadParameter("join method %q does not support scoping", token.GetSpec().GetJoinMethod())
	}

	return nil
}

// StrongValidateToken checks if the scoped token is well-formed according to
// all scoped token rules. This function *must* be used to validate any scoped
// token being created from scratch. When validating existing scoped token
// resources, this function should be avoided in favor of the
// [WeakValidateToken] function.
func StrongValidateToken(token *joiningv1.ScopedToken) error {
	if expected, actual := types.KindScopedToken, token.GetKind(); expected != actual {
		return trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := types.V1, token.GetVersion(); expected != actual {
		return trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", token.GetSubKind(); expected != actual {
		return trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := token.GetMetadata().GetName(); name == "" {
		return trace.BadParameter("missing name")
	}

	if token.GetScope() == "" {
		return trace.BadParameter("scoped token must have a scope assigned")
	}

	spec := token.GetSpec()
	if spec == nil {
		return trace.BadParameter("spec must not be nil")
	}

	if err := scopes.StrongValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if err := scopes.StrongValidate(spec.AssignedScope); err != nil {
		return trace.Wrap(err, "validating scoped token assigned scope")
	}

	if !scopes.ResourceScope(spec.AssignedScope).IsSubjectToPolicyScope(token.GetScope()) {
		return trace.BadParameter("scoped token assigned scope must be descendant of its resource scope")
	}

	if err := validateJoinMethod(token); err != nil {
		return trace.Wrap(err)
	}

	switch TokenUsageMode(spec.GetUsageMode()) {
	case TokenUsageModeSingle, TokenUsageModeUnlimited:
	default:
		return trace.BadParameter("scoped token mode is not supported")
	}

	if len(spec.Roles) == 0 {
		return trace.BadParameter("scoped token must have at least one role")
	}

	roles, err := types.NewTeleportRoles(spec.Roles)
	if err != nil {
		return trace.Wrap(err, "validating scoped token roles")
	}

	if err := validateImmutableLabels(spec); err != nil {
		return trace.Wrap(err)
	}

	for _, role := range roles {
		if !rolesSupportingScopes.Include(role) {
			return trace.BadParameter("role %q does not support scoping", role)
		}
	}

	return nil
}

// WeakValidateToken performs a weak form of validation on a scoped token. This
// function is intended to catch bugs/incompatibilites that might have resulted
// in a scoped token too malformed for us to safely reason about (e.g. due to
// significant version drift). Use this function to validate scoped tokens
// propagated from the control plane. Prefer using [StrongValidateToken] when
// building a new scoped token from scratch.
func WeakValidateToken(token *joiningv1.ScopedToken) error {
	if token == nil {
		return trace.BadParameter("missing scoped token")
	}

	if err := scopes.WeakValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if err := scopes.WeakValidate(token.GetSpec().GetAssignedScope()); err != nil {
		return trace.Wrap(err, "validating scoped token assigned scope")
	}

	if len(token.GetSpec().GetRoles()) == 0 {
		return trace.BadParameter("scoped token must have at least one role")
	}

	if err := validateJoinMethod(token); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var ErrTokenExpired = &trace.LimitExceededError{Message: "scoped token is expired"}

var ErrTokenExhausted = &trace.LimitExceededError{Message: "scoped token usage exhausted"}

// ValidateTokenForUse checks if a given scoped token can be used for
// provisioning. Returns a [*trace.LimitExceededError] if the token is expired
func ValidateTokenForUse(token *joiningv1.ScopedToken) error {
	if err := WeakValidateToken(token); err != nil {
		return trace.Wrap(err)
	}

	now := time.Now().UTC()
	ttl := token.GetMetadata().GetExpires()
	if ttl != nil && !ttl.AsTime().IsZero() {
		if ttl.AsTime().Before(now) {
			return trace.Wrap(ErrTokenExpired)
		}
	}

	reusableUntil := token.GetStatus().GetUsage().GetSingleUse().GetReusableUntil()
	if reusableUntil != nil && !reusableUntil.AsTime().IsZero() {
		if reusableUntil.AsTime().Before(now) {
			return trace.Wrap(ErrTokenExhausted)
		}
	}

	return nil
}

// ValidateTokenUpdate checks for invalid updates between two tokens.
// If the scope, usage mode, or secret was changed between two token updates,
// a trace.BadParameter error is returned.
func ValidateTokenUpdate(oldToken *joiningv1.ScopedToken, newToken *joiningv1.ScopedToken) error {
	if newToken == nil {
		return trace.BadParameter("new token is invalid")
	}
	// no old token to compare to so we assume that the new token is valid and no need for additional checks
	if oldToken == nil {
		return nil
	}
	tokenName := newToken.GetMetadata().GetName()
	if oldToken.GetScope() != newToken.GetScope() {
		return trace.BadParameter("cannot modify scope of existing scoped token %s with scope %s to %s", tokenName, oldToken.GetScope(), newToken.GetScope())
	}

	if oldToken.GetSpec().GetUsageMode() != newToken.GetSpec().GetUsageMode() {
		return trace.BadParameter("cannot modify usage mode of existing scoped token %s from usage mode %s to %s", tokenName, oldToken.GetSpec().GetUsageMode(), newToken.GetSpec().GetUsageMode())
	}

	if oldToken.GetStatus().GetSecret() != newToken.GetStatus().GetSecret() {
		return trace.BadParameter("cannot modify secret of existing scoped token %s", tokenName)
	}

	return nil
}

// Token wraps a [joiningv1.ScopedToken] such that it can be used to provision
// resources.
type Token struct {
	scoped     *joiningv1.ScopedToken
	joinMethod types.JoinMethod
	roles      types.SystemRoles
}

// NewToken returns the wrapped version of the given [joiningv1.ScopedToken].
// It will return an error if the configured join method is not a valid
// [types.JoinMethod] or if any of the configured roles are not a valid
// [types.SystemRole]. The validated join method and roles are cached on the
// [Scoped] wrapper itself so they can be read without repeating validation.
func NewToken(token *joiningv1.ScopedToken) (*Token, error) {
	joinMethod := types.JoinMethod(token.GetSpec().GetJoinMethod())
	if err := types.ValidateJoinMethod(joinMethod); err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := types.NewTeleportRoles(token.GetSpec().GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Token{scoped: token, joinMethod: joinMethod, roles: roles}, nil
}

// GetName returns the name of a [joiningv1.ScopedToken].
func (t *Token) GetName() string {
	if t == nil {
		return ""
	}

	return t.scoped.GetMetadata().GetName()
}

// GetJoinMethod returns the cached [types.JoinMethod] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (t *Token) GetJoinMethod() types.JoinMethod {
	if t == nil {
		return types.JoinMethodUnspecified
	}

	return t.joinMethod
}

// GetRoles returns the cached [types.SystemRoles] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (t *Token) GetRoles() types.SystemRoles {
	if t == nil {
		return nil
	}
	return t.roles
}

// GetSafeName returns the name the santiized name of the scoped token. Because
// scoped token names are not secret, this is just an alias for [GetName].
func (t *Token) GetSafeName() string {
	return t.GetName()
}

// Expiry returns the [time.Time] representing when the wrapped
// [joiningv1.ScopedToken] will expire.
func (t *Token) Expiry() time.Time {
	expiry := t.scoped.GetMetadata().GetExpires()
	if expiry == nil {
		return time.Time{}
	}

	return expiry.AsTime()
}

// GetBotName returns an empty string because scoped tokens do not currently
// support configuring a bot name.
func (t *Token) GetBotName() string {
	return ""
}

// GetAssignedScope returns the scope that will be assigned to resources
// provisioned using the wrapped [joiningv1.ScopedToken].
func (t *Token) GetAssignedScope() string {
	return t.scoped.GetSpec().GetAssignedScope()
}

// GetSecret returns the token's secret value.
func (t *Token) GetSecret() (string, bool) {
	return t.scoped.GetStatus().GetSecret(), t.GetJoinMethod() == types.JoinMethodToken
}

// GetAllowRules returns the list of allow rules.
func (t *Token) GetAWSAllowRules() []*types.TokenRule {
	allow := make([]*types.TokenRule, len(t.scoped.GetSpec().GetAws().GetAllow()))
	for i, rule := range t.scoped.GetSpec().GetAws().GetAllow() {
		allow[i] = &types.TokenRule{
			AWSAccount:        rule.GetAwsAccount(),
			AWSRegions:        rule.GetAwsRegions(),
			AWSRole:           rule.GetAwsRole(),
			AWSARN:            rule.GetAwsArn(),
			AWSOrganizationID: rule.GetAwsOrganizationId(),
		}
	}

	return allow
}

// GetAWSIIDTTL returns the TTL of EC2 IIDs
func (t *Token) GetAWSIIDTTL() types.Duration {
	ttl := t.scoped.GetSpec().GetAws().GetAwsIidTtl()
	if ttl == 0 {
		// default to 5 minute ttl if unspecified
		return types.Duration(5 * time.Minute)
	}
	return types.Duration(ttl)
}

// GetIntegration returns the Integration field which is used to provide
// credentials that will be used when validating the AWS Organization if required by an IAM Token.
func (t *Token) GetIntegration() string {
	return t.scoped.GetSpec().GetAws().GetIntegration()
}

// GetGCPRules returns the GCP-specific configuration for this token.
func (t *Token) GetGCPRules() *types.ProvisionTokenSpecV2GCP {
	allow := make([]*types.ProvisionTokenSpecV2GCP_Rule, len(t.scoped.GetSpec().GetGcp().GetAllow()))
	for i, rule := range t.scoped.GetSpec().GetGcp().GetAllow() {
		allow[i] = &types.ProvisionTokenSpecV2GCP_Rule{
			ProjectIDs:      rule.GetProjectIds(),
			Locations:       rule.GetLocations(),
			ServiceAccounts: rule.GetServiceAccounts(),
		}
	}

	return &types.ProvisionTokenSpecV2GCP{
		Allow: allow,
	}
}

// GetAzure returns the Azure-specific configuration for this token.
func (t *Token) GetAzure() *types.ProvisionTokenSpecV2Azure {
	allow := make([]*types.ProvisionTokenSpecV2Azure_Rule, len(t.scoped.GetSpec().GetAzure().GetAllow()))
	for i, rule := range t.scoped.GetSpec().GetAzure().GetAllow() {
		allow[i] = &types.ProvisionTokenSpecV2Azure_Rule{
			Subscription:   rule.GetSubscription(),
			ResourceGroups: rule.GetResourceGroups(),
		}
	}

	return &types.ProvisionTokenSpecV2Azure{
		Allow: allow,
	}
}

// GetAzureDevops returns the AzureDevops-specific configuration for this token.
func (t *Token) GetAzureDevops() *types.ProvisionTokenSpecV2AzureDevops {
	allow := make([]*types.ProvisionTokenSpecV2AzureDevops_Rule, len(t.scoped.GetSpec().GetAzureDevops().GetAllow()))
	for i, rule := range t.scoped.GetSpec().GetAzureDevops().GetAllow() {
		allow[i] = &types.ProvisionTokenSpecV2AzureDevops_Rule{
			Sub:               rule.GetSub(),
			ProjectName:       rule.GetProjectName(),
			PipelineName:      rule.GetPipelineName(),
			ProjectID:         rule.GetProjectId(),
			DefinitionID:      rule.GetDefinitionId(),
			RepositoryURI:     rule.GetRepositoryUri(),
			RepositoryVersion: rule.GetRepositoryVersion(),
			RepositoryRef:     rule.GetRepositoryRef(),
		}
	}

	return &types.ProvisionTokenSpecV2AzureDevops{
		Allow:          allow,
		OrganizationID: t.scoped.GetSpec().GetAzureDevops().GetOrganizationId(),
	}
}

// GetOracle returns the Oracle-specific configuration for this token.
func (t *Token) GetOracle() *types.ProvisionTokenSpecV2Oracle {
	allow := make([]*types.ProvisionTokenSpecV2Oracle_Rule, len(t.scoped.GetSpec().GetOracle().GetAllow()))
	for i, rule := range t.scoped.GetSpec().GetOracle().GetAllow() {
		allow[i] = &types.ProvisionTokenSpecV2Oracle_Rule{
			Tenancy:            rule.GetTenancy(),
			ParentCompartments: rule.GetParentCompartments(),
			Regions:            rule.GetRegions(),
			Instances:          rule.GetInstances(),
		}
	}

	return &types.ProvisionTokenSpecV2Oracle{
		Allow: allow,
	}
}

// GetScopedToken attempts to return the underlying [*joiningv1.ScopedToken] backing a
// [provision.Token]. Returns a boolean indicating whether the token is scoped or not.
func GetScopedToken(token provision.Token) (*joiningv1.ScopedToken, bool) {
	wrapper, ok := token.(*Token)
	if !ok {
		return nil, false
	}

	return wrapper.scoped, true
}

// GetImmutableLabels returns labels that must be applied to resources
// provisioned with this token.
func (t *Token) GetImmutableLabels() *joiningv1.ImmutableLabels {
	return t.scoped.GetSpec().GetImmutableLabels()
}

func validateImmutableLabels(spec *joiningv1.ScopedTokenSpec) error {
	if spec == nil {
		return nil
	}

	sshLabels := spec.GetImmutableLabels().GetSsh()
	if len(sshLabels) > 0 {
		if !slices.Contains(spec.GetRoles(), string(types.RoleNode)) {
			return trace.BadParameter("immutable ssh labels are only supported for tokens that allow the node role")
		}
	}

	for k := range sshLabels {
		if !types.IsValidLabelKey(k) {
			return trace.BadParameter("invalid immutable label key %q", k)
		}
	}

	return nil
}

// HashImmutableLabels returns a deterministic hash of the given [*joiningv1.ImmutableLabels].
func HashImmutableLabels(labels *joiningv1.ImmutableLabels) string {
	if labels == nil {
		return ""
	}

	hash := sha256.New()
	var bytesWrittenToHash int
	writeHash := func(p []byte) {
		n, _ := hash.Write(p)
		bytesWrittenToHash += n
	}

	if sshLabels := labels.GetSsh(); len(sshLabels) > 0 {
		sorted := make([]struct{ key, value string }, 0, len(sshLabels))
		for k, v := range sshLabels {
			sorted = append(sorted, struct{ key, value string }{k, v})
		}
		slices.SortFunc(sorted, func(a, b struct{ key, value string }) int {
			return cmp.Compare(a.key, b.key)
		})

		// first we write the service type so that the following labels do not collide with identical labels
		// from other services e.g. app labels or database labels
		writeHash([]byte("ssh"))

		// Each map entry is added to the hash as 4 components:
		// 1. The length of the key
		// 2. The value of the key
		// 3. The length of the value
		// 4. The value itself
		// This combination prevents collisions between:
		// - single labels (e.g. aaa=bbb and aaab=bb)
		// - splitting labels (e.g. aaa=bbbcccddd and aaa=bbb,ccc=ddd)
		// ...because in both cases the lengths of the keys/values must change to create different labels from
		// the same set of characters.
		for _, v := range sorted {
			buf := [8]byte{}
			binary.BigEndian.PutUint64(buf[:], uint64(len(v.key)))
			writeHash(buf[:])
			writeHash([]byte(v.key))
			binary.BigEndian.PutUint64(buf[:], uint64(len(v.value)))
			writeHash(buf[:])
			writeHash([]byte(v.value))
		}
	}

	if bytesWrittenToHash == 0 {
		return ""
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// VerifyImmutableLabelsHash returns whether or not the given [*joiningv1.ImmutableLabels]
// matches the given hash.
func VerifyImmutableLabelsHash(labels *joiningv1.ImmutableLabels, hash string) bool {
	newHash := HashImmutableLabels(labels)
	return newHash == hash
}
