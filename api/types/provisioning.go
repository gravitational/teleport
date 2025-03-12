/*
Copyright 2020-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// JoinMethod is the method used for new nodes to join the cluster.
type JoinMethod string

const (
	JoinMethodUnspecified JoinMethod = ""
	// JoinMethodToken is the default join method, nodes join the cluster by
	// presenting a secret token.
	JoinMethodToken JoinMethod = "token"
	// JoinMethodEC2 indicates that the node will join with the EC2 join method.
	JoinMethodEC2 JoinMethod = "ec2"
	// JoinMethodIAM indicates that the node will join with the IAM join method.
	JoinMethodIAM JoinMethod = "iam"
	// JoinMethodGitHub indicates that the node will join with the GitHub join
	// method. Documentation regarding the implementation of this can be found
	// in lib/githubactions
	JoinMethodGitHub JoinMethod = "github"
	// JoinMethodCircleCI indicates that the node will join with the CircleCI\
	// join method. Documentation regarding the implementation of this can be
	// found in lib/circleci
	JoinMethodCircleCI JoinMethod = "circleci"
	// JoinMethodKubernetes indicates that the node will join with the
	// Kubernetes join method. Documentation regarding implementation can be
	// found in lib/kubernetestoken
	JoinMethodKubernetes JoinMethod = "kubernetes"
	// JoinMethodAzure indicates that the node will join with the Azure join
	// method.
	JoinMethodAzure JoinMethod = "azure"
	// JoinMethodGitLab indicates that the node will join with the GitLab
	// join method. Documentation regarding implementation of this
	// can be found in lib/gitlab
	JoinMethodGitLab JoinMethod = "gitlab"
	// JoinMethodGCP indicates that the node will join with the GCP join method.
	// Documentation regarding implementation of this can be found in lib/gcp.
	JoinMethodGCP JoinMethod = "gcp"
	// JoinMethodSpacelift indicates the node will join with the SpaceLift join
	// method. Documentation regarding implementation of this can be found in
	// lib/spacelift.
	JoinMethodSpacelift JoinMethod = "spacelift"
	// JoinMethodTPM indicates that the node will join with the TPM join method.
	// The core implementation of this join method can be found in lib/tpm.
	JoinMethodTPM JoinMethod = "tpm"
	// JoinMethodTerraformCloud indicates that the node will join using the Terraform
	// join method. See lib/terraformcloud for more.
	JoinMethodTerraformCloud JoinMethod = "terraform_cloud"
	// JoinMethodBitbucket indicates that the node will join using the Bitbucket
	// join method. See lib/bitbucket for more.
	JoinMethodBitbucket JoinMethod = "bitbucket"
	// JoinMethodOracle indicates that the node will join using the Oracle join
	// method.
	JoinMethodOracle JoinMethod = "oracle"
)

var JoinMethods = []JoinMethod{
	JoinMethodAzure,
	JoinMethodBitbucket,
	JoinMethodCircleCI,
	JoinMethodEC2,
	JoinMethodGCP,
	JoinMethodGitHub,
	JoinMethodGitLab,
	JoinMethodIAM,
	JoinMethodKubernetes,
	JoinMethodSpacelift,
	JoinMethodToken,
	JoinMethodTPM,
	JoinMethodTerraformCloud,
	JoinMethodOracle,
}

func ValidateJoinMethod(method JoinMethod) error {
	hasJoinMethod := slices.Contains(JoinMethods, method)
	if !hasJoinMethod {
		return trace.BadParameter("join method must be one of %s", apiutils.JoinStrings(JoinMethods, ", "))
	}

	return nil
}

type KubernetesJoinType string

var (
	KubernetesJoinTypeUnspecified KubernetesJoinType = ""
	KubernetesJoinTypeInCluster   KubernetesJoinType = "in_cluster"
	KubernetesJoinTypeStaticJWKS  KubernetesJoinType = "static_jwks"
)

// ProvisionToken is a provisioning token
type ProvisionToken interface {
	ResourceWithOrigin
	// SetMetadata sets resource metatada
	SetMetadata(meta Metadata)
	// GetRoles returns a list of teleport roles
	// that will be granted to the user of the token
	// in the crendentials
	GetRoles() SystemRoles
	// SetRoles sets teleport roles
	SetRoles(SystemRoles)
	// SetLabels sets the tokens labels
	SetLabels(map[string]string)
	// GetAllowRules returns the list of allow rules
	GetAllowRules() []*TokenRule
	// SetAllowRules sets the allow rules
	SetAllowRules([]*TokenRule)
	// GetGCPRules will return the GCP rules within this token.
	GetGCPRules() *ProvisionTokenSpecV2GCP
	// GetAWSIIDTTL returns the TTL of EC2 IIDs
	GetAWSIIDTTL() Duration
	// GetJoinMethod returns joining method that must be used with this token.
	GetJoinMethod() JoinMethod
	// GetBotName returns the BotName field which must be set for joining bots.
	GetBotName() string
	// IsStatic returns true if the token is statically configured
	IsStatic() bool
	// GetSuggestedLabels returns the set of labels that the resource should add when adding itself to the cluster
	GetSuggestedLabels() Labels

	// GetSuggestedAgentMatcherLabels returns the set of labels that should be watched when an agent/service uses this token.
	// An example of this is the Database Agent.
	// When using the install-database.sh script, the script will add those labels as part of the `teleport.yaml` configuration.
	// They are added to `db_service.resources.0.labels`.
	GetSuggestedAgentMatcherLabels() Labels

	// V1 returns V1 version of the resource
	V1() *ProvisionTokenV1
	// String returns user friendly representation of the resource
	String() string

	// GetSafeName returns the name of the token, sanitized appropriately for
	// join methods where the name is secret. This should be used when logging
	// the token name.
	GetSafeName() string
}

// NewProvisionToken returns a new provision token with the given roles.
func NewProvisionToken(token string, roles SystemRoles, expires time.Time) (ProvisionToken, error) {
	return NewProvisionTokenFromSpec(token, expires, ProvisionTokenSpecV2{
		Roles: roles,
	})
}

// NewProvisionTokenFromSpec returns a new provision token with the given spec.
func NewProvisionTokenFromSpec(token string, expires time.Time, spec ProvisionTokenSpecV2) (ProvisionToken, error) {
	t := &ProvisionTokenV2{
		Metadata: Metadata{
			Name:    token,
			Expires: &expires,
		},
		Spec: spec,
	}
	if err := t.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return t, nil
}

// MustCreateProvisionToken returns a new valid provision token
// or panics, used in tests
func MustCreateProvisionToken(token string, roles SystemRoles, expires time.Time) ProvisionToken {
	t, err := NewProvisionToken(token, roles, expires)
	if err != nil {
		panic(err)
	}
	return t
}

// setStaticFields sets static resource header and metadata fields.
func (p *ProvisionTokenV2) setStaticFields() {
	p.Kind = KindToken
	p.Version = V2
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (p *ProvisionTokenV2) CheckAndSetDefaults() error {
	p.setStaticFields()
	if err := p.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if len(p.Spec.Roles) == 0 {
		return trace.BadParameter("provisioning token is missing roles")
	}
	roles, err := NewTeleportRoles(SystemRoles(p.Spec.Roles).StringSlice())
	if err != nil {
		return trace.Wrap(err)
	}
	p.Spec.Roles = roles

	if roles.Include(RoleBot) && p.Spec.BotName == "" {
		return trace.BadParameter("token with role %q must set bot_name", RoleBot)
	}

	if p.Spec.BotName != "" && !roles.Include(RoleBot) {
		return trace.BadParameter("can only set bot_name on token with role %q", RoleBot)
	}

	hasAllowRules := len(p.Spec.Allow) > 0
	if p.Spec.JoinMethod == JoinMethodUnspecified {
		// Default to the ec2 join method if any allow rules were specified,
		// else default to the token method. These defaults are necessary for
		// backwards compatibility.
		if hasAllowRules {
			p.Spec.JoinMethod = JoinMethodEC2
		} else {
			p.Spec.JoinMethod = JoinMethodToken
		}
	}
	switch p.Spec.JoinMethod {
	case JoinMethodToken:
		if hasAllowRules {
			return trace.BadParameter("allow rules are not compatible with the %q join method", JoinMethodToken)
		}
	case JoinMethodEC2:
		if !hasAllowRules {
			return trace.BadParameter("the %q join method requires defined token allow rules", JoinMethodEC2)
		}
		for _, allowRule := range p.Spec.Allow {
			if allowRule.AWSARN != "" {
				return trace.BadParameter(`the %q join method does not support the "aws_arn" parameter`, JoinMethodEC2)
			}
			if allowRule.AWSAccount == "" && allowRule.AWSRole == "" {
				return trace.BadParameter(`allow rule for %q join method must set "aws_account" or "aws_role"`, JoinMethodEC2)
			}
		}
		if p.Spec.AWSIIDTTL == 0 {
			// default to 5 minute ttl if unspecified
			p.Spec.AWSIIDTTL = Duration(5 * time.Minute)
		}
	case JoinMethodIAM:
		if !hasAllowRules {
			return trace.BadParameter("the %q join method requires defined token allow rules", JoinMethodIAM)
		}
		for _, allowRule := range p.Spec.Allow {
			if allowRule.AWSRole != "" {
				return trace.BadParameter(`the %q join method does not support the "aws_role" parameter`, JoinMethodIAM)
			}
			if len(allowRule.AWSRegions) != 0 {
				return trace.BadParameter(`the %q join method does not support the "aws_regions" parameter`, JoinMethodIAM)
			}
			if allowRule.AWSAccount == "" && allowRule.AWSARN == "" {
				return trace.BadParameter(`allow rule for %q join method must set "aws_account" or "aws_arn"`, JoinMethodEC2)
			}
		}
	case JoinMethodGitHub:
		providerCfg := p.Spec.GitHub
		if providerCfg == nil {
			return trace.BadParameter(
				`"github" configuration must be provided for join method %q`,
				JoinMethodGitHub,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case JoinMethodCircleCI:
		providerCfg := p.Spec.CircleCI
		if providerCfg == nil {
			return trace.BadParameter(
				`"cirleci" configuration must be provided for join method %q`,
				JoinMethodCircleCI,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case JoinMethodKubernetes:
		providerCfg := p.Spec.Kubernetes
		if providerCfg == nil {
			return trace.BadParameter(
				`"kubernetes" configuration must be provided for the join method %q`,
				JoinMethodKubernetes,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err, "spec.kubernetes:")
		}
	case JoinMethodAzure:
		providerCfg := p.Spec.Azure
		if providerCfg == nil {
			return trace.BadParameter(
				`"azure" configuration must be provided for the join method %q`,
				JoinMethodAzure,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case JoinMethodGitLab:
		providerCfg := p.Spec.GitLab
		if providerCfg == nil {
			return trace.BadParameter(
				`"gitlab" configuration must be provided for the join method %q`,
				JoinMethodGitLab,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case JoinMethodGCP:
		providerCfg := p.Spec.GCP
		if providerCfg == nil {
			return trace.BadParameter(
				`"gcp" configuration must be provided for the join method %q`,
				JoinMethodGCP,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case JoinMethodSpacelift:
		providerCfg := p.Spec.Spacelift
		if providerCfg == nil {
			return trace.BadParameter(
				`spec.spacelift: must be configured for the join method %q`,
				JoinMethodSpacelift,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err, "spec.spacelift: failed validation")
		}
	case JoinMethodTPM:
		providerCfg := p.Spec.TPM
		if providerCfg == nil {
			return trace.BadParameter(
				`spec.tpm: must be configured for the join method %q`,
				JoinMethodTPM,
			)
		}
		if err := providerCfg.validate(); err != nil {
			return trace.Wrap(err, "spec.tpm: failed validation")
		}
	case JoinMethodTerraformCloud:
		providerCfg := p.Spec.TerraformCloud
		if providerCfg == nil {
			return trace.BadParameter(
				"spec.terraform_cloud: must be configured for the join method %q",
				JoinMethodTerraformCloud,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err, "spec.terraform_cloud: failed validation")
		}
	case JoinMethodBitbucket:
		providerCfg := p.Spec.Bitbucket
		if providerCfg == nil {
			return trace.BadParameter(
				"spec.bitbucket: must be configured for the join method %q",
				JoinMethodBitbucket,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err, "spec.bitbucket: failed validation")
		}
	case JoinMethodOracle:
		providerCfg := p.Spec.Oracle
		if providerCfg == nil {
			return trace.BadParameter(
				"spec.oracle: must be configured for the join method %q",
				JoinMethodOracle,
			)
		}
		if err := providerCfg.checkAndSetDefaults(); err != nil {
			return trace.Wrap(err, "spec.oracle: failed validation")
		}
	default:
		return trace.BadParameter("unknown join method %q", p.Spec.JoinMethod)
	}

	return nil
}

// GetVersion returns resource version
func (p *ProvisionTokenV2) GetVersion() string {
	return p.Version
}

// GetRoles returns a list of teleport roles
// that will be granted to the user of the token
// in the crendentials
func (p *ProvisionTokenV2) GetRoles() SystemRoles {
	// Ensure that roles are case-insensitive.
	return normalizedSystemRoles(SystemRoles(p.Spec.Roles).StringSlice())
}

// SetRoles sets teleport roles
func (p *ProvisionTokenV2) SetRoles(r SystemRoles) {
	p.Spec.Roles = r
}

func (p *ProvisionTokenV2) SetLabels(l map[string]string) {
	p.Metadata.Labels = l
}

// GetAllowRules returns the list of allow rules
func (p *ProvisionTokenV2) GetAllowRules() []*TokenRule {
	return p.Spec.Allow
}

// SetAllowRules sets the allow rules.
func (p *ProvisionTokenV2) SetAllowRules(rules []*TokenRule) {
	p.Spec.Allow = rules
}

// GetGCPRules will return the GCP rules within this token.
func (p *ProvisionTokenV2) GetGCPRules() *ProvisionTokenSpecV2GCP {
	return p.Spec.GCP
}

// GetAWSIIDTTL returns the TTL of EC2 IIDs
func (p *ProvisionTokenV2) GetAWSIIDTTL() Duration {
	return p.Spec.AWSIIDTTL
}

// GetJoinMethod returns joining method that must be used with this token.
func (p *ProvisionTokenV2) GetJoinMethod() JoinMethod {
	return p.Spec.JoinMethod
}

// IsStatic returns true if the token is statically configured
func (p *ProvisionTokenV2) IsStatic() bool {
	return p.Origin() == OriginConfigFile
}

// GetBotName returns the BotName field which must be set for joining bots.
func (p *ProvisionTokenV2) GetBotName() string {
	return p.Spec.BotName
}

// GetKind returns resource kind
func (p *ProvisionTokenV2) GetKind() string {
	return p.Kind
}

// GetSubKind returns resource sub kind
func (p *ProvisionTokenV2) GetSubKind() string {
	return p.SubKind
}

// SetSubKind sets resource subkind
func (p *ProvisionTokenV2) SetSubKind(s string) {
	p.SubKind = s
}

// GetRevision returns the revision
func (p *ProvisionTokenV2) GetRevision() string {
	return p.Metadata.GetRevision()
}

// SetRevision sets the revision
func (p *ProvisionTokenV2) SetRevision(rev string) {
	p.Metadata.SetRevision(rev)
}

// GetMetadata returns metadata
func (p *ProvisionTokenV2) GetMetadata() Metadata {
	return p.Metadata
}

// SetMetadata sets resource metatada
func (p *ProvisionTokenV2) SetMetadata(meta Metadata) {
	p.Metadata = meta
}

// Origin returns the origin value of the resource.
func (p *ProvisionTokenV2) Origin() string {
	return p.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (p *ProvisionTokenV2) SetOrigin(origin string) {
	p.Metadata.SetOrigin(origin)
}

// GetSuggestedLabels returns the labels the resource should set when using this token
func (p *ProvisionTokenV2) GetSuggestedLabels() Labels {
	return p.Spec.SuggestedLabels
}

// GetAgentMatcherLabels returns the set of labels that should be watched when an agent/service uses this token.
// An example of this is the Database Agent.
// When using the install-database.sh script, the script will add those labels as part of the `teleport.yaml` configuration.
// They are added to `db_service.resources.0.labels`.
func (p *ProvisionTokenV2) GetSuggestedAgentMatcherLabels() Labels {
	return p.Spec.SuggestedAgentMatcherLabels
}

// V1 returns V1 version of the resource
func (p *ProvisionTokenV2) V1() *ProvisionTokenV1 {
	return &ProvisionTokenV1{
		Roles:   p.Spec.Roles,
		Expires: p.Metadata.Expiry(),
		Token:   p.Metadata.Name,
	}
}

// V2 returns V2 version of the resource
func (p *ProvisionTokenV2) V2() *ProvisionTokenV2 {
	return p
}

// SetExpiry sets expiry time for the object
func (p *ProvisionTokenV2) SetExpiry(expires time.Time) {
	p.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (p *ProvisionTokenV2) Expiry() time.Time {
	return p.Metadata.Expiry()
}

// GetName returns the name of the provision token. This value can be secret!
// Use GetSafeName where the name may be logged.
func (p *ProvisionTokenV2) GetName() string {
	return p.Metadata.Name
}

// SetName sets the name of the provision token.
func (p *ProvisionTokenV2) SetName(e string) {
	p.Metadata.Name = e
}

// GetSafeName returns the name of the token, sanitized appropriately for
// join methods where the name is secret. This should be used when logging
// the token name.
func (p *ProvisionTokenV2) GetSafeName() string {
	name := p.GetName()
	if p.GetJoinMethod() != JoinMethodToken {
		return name
	}

	// If the token name is short, we just blank the whole thing.
	if len(name) < 16 {
		return strings.Repeat("*", len(name))
	}

	// If the token name is longer, we can show the last 25% of it to help
	// the operator identify it.
	hiddenBefore := int(0.75 * float64(len(name)))
	name = name[hiddenBefore:]
	name = strings.Repeat("*", hiddenBefore) + name
	return name
}

// String returns the human readable representation of a provisioning token.
func (p ProvisionTokenV2) String() string {
	expires := "never"
	if !p.Expiry().IsZero() {
		expires = p.Expiry().String()
	}
	return fmt.Sprintf("ProvisionToken(Roles=%v, Expires=%v)", p.Spec.Roles, expires)
}

// ProvisionTokensToV1 converts provision tokens to V1 list
func ProvisionTokensToV1(in []ProvisionToken) []ProvisionTokenV1 {
	if in == nil {
		return nil
	}
	out := make([]ProvisionTokenV1, len(in))
	for i := range in {
		out[i] = *in[i].V1()
	}
	return out
}

// ProvisionTokensFromStatic converts static tokens to resource list
func ProvisionTokensFromStatic(in []ProvisionTokenV1) []ProvisionToken {
	if in == nil {
		return nil
	}
	out := make([]ProvisionToken, len(in))
	for i := range in {
		tok := in[i].V2()
		tok.SetOrigin(OriginConfigFile)
		out[i] = tok
	}
	return out
}

// V1 returns V1 version of the resource
func (p *ProvisionTokenV1) V1() *ProvisionTokenV1 {
	return p
}

// V2 returns V2 version of the resource
func (p *ProvisionTokenV1) V2() *ProvisionTokenV2 {
	t := &ProvisionTokenV2{
		Kind:    KindToken,
		Version: V2,
		Metadata: Metadata{
			Name:      p.Token,
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV2{
			Roles: p.Roles,
		},
	}
	if !p.Expires.IsZero() {
		t.SetExpiry(p.Expires)
	}
	t.CheckAndSetDefaults()
	return t
}

// String returns the human readable representation of a provisioning token.
func (p ProvisionTokenV1) String() string {
	expires := "never"
	if p.Expires.Unix() != 0 {
		expires = p.Expires.String()
	}
	return fmt.Sprintf("ProvisionToken(Roles=%v, Expires=%v)",
		p.Roles, expires)
}

func (a *ProvisionTokenSpecV2GitHub) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one token allow rule", JoinMethodGitHub)
	}
	for _, rule := range a.Allow {
		repoSet := rule.Repository != ""
		ownerSet := rule.RepositoryOwner != ""
		subSet := rule.Sub != ""
		if !(subSet || ownerSet || repoSet) {
			return trace.BadParameter(
				`allow rule for %q must include at least one of "repository", "repository_owner" or "sub"`,
				JoinMethodGitHub,
			)
		}
	}
	if strings.Contains(a.EnterpriseServerHost, "/") {
		return trace.BadParameter("'spec.github.enterprise_server_host' should not contain the scheme or path")
	}
	if a.EnterpriseServerHost != "" && a.EnterpriseSlug != "" {
		return trace.BadParameter("'spec.github.enterprise_server_host' and `spec.github.enterprise_slug` cannot both be set")
	}
	return nil
}

func (a *ProvisionTokenSpecV2CircleCI) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one token allow rule", JoinMethodCircleCI)
	}
	if a.OrganizationID == "" {
		return trace.BadParameter("the %q join method requires 'organization_id' to be set", JoinMethodCircleCI)
	}
	for _, rule := range a.Allow {
		projectSet := rule.ProjectID != ""
		contextSet := rule.ContextID != ""
		if !projectSet && !contextSet {
			return trace.BadParameter(
				`allow rule for %q must include at least "project_id" or "context_id"`,
				JoinMethodCircleCI,
			)
		}
	}
	return nil
}

func (a *ProvisionTokenSpecV2Kubernetes) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("allow: at least one rule must be set")
	}
	for i, allowRule := range a.Allow {
		if allowRule.ServiceAccount == "" {
			return trace.BadParameter(
				"allow[%d].service_account: name of service account must be set",
				i,
			)
		}
		if len(strings.Split(allowRule.ServiceAccount, ":")) != 2 {
			return trace.BadParameter(
				`allow[%d].service_account: name of service account should be in format "namespace:service_account", got %q instead`,
				i,
				allowRule.ServiceAccount,
			)
		}
	}

	if a.Type == KubernetesJoinTypeUnspecified {
		// For compatibility with older resources which did not have a Type
		// field we default to "in_cluster".
		a.Type = KubernetesJoinTypeInCluster
	}
	switch a.Type {
	case KubernetesJoinTypeInCluster:
		if a.StaticJWKS != nil {
			return trace.BadParameter("static_jwks: must not be set when type is %q", KubernetesJoinTypeInCluster)
		}
	case KubernetesJoinTypeStaticJWKS:
		if a.StaticJWKS == nil {
			return trace.BadParameter("static_jwks: must be set when type is %q", KubernetesJoinTypeStaticJWKS)
		}
		if a.StaticJWKS.JWKS == "" {
			return trace.BadParameter("static_jwks.jwks: must be set when type is %q", KubernetesJoinTypeStaticJWKS)
		}
	default:
		return trace.BadParameter(
			"type: must be one of (%s), got %q",
			apiutils.JoinStrings(JoinMethods, ", "),
			a.Type,
		)
	}

	return nil
}

func (a *ProvisionTokenSpecV2Azure) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter(
			"the %q join method requires defined azure allow rules",
			JoinMethodAzure,
		)
	}
	for _, allowRule := range a.Allow {
		if allowRule.Subscription == "" {
			return trace.BadParameter(
				"the %q join method requires azure allow rules with non-empty subscription",
				JoinMethodAzure,
			)
		}
	}
	return nil
}

const defaultGitLabDomain = "gitlab.com"

func (a *ProvisionTokenSpecV2GitLab) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter(
			"the %q join method requires defined gitlab allow rules",
			JoinMethodGitLab,
		)
	}
	for _, allowRule := range a.Allow {
		if allowRule.Sub == "" && allowRule.NamespacePath == "" && allowRule.ProjectPath == "" && allowRule.CIConfigRefURI == "" {
			return trace.BadParameter(
				"the %q join method requires allow rules with at least one of ['sub', 'project_path', 'namespace_path', 'ci_config_ref_uri'] to ensure security.",
				JoinMethodGitLab,
			)
		}
	}

	if a.Domain == "" {
		a.Domain = defaultGitLabDomain
	} else {
		if strings.Contains(a.Domain, "/") {
			return trace.BadParameter(
				"'spec.gitlab.domain' should not contain the scheme or path",
			)
		}
	}
	return nil
}

func (a *ProvisionTokenSpecV2GCP) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one token allow rule", JoinMethodGCP)
	}
	for _, allowRule := range a.Allow {
		if len(allowRule.ProjectIDs) == 0 {
			return trace.BadParameter(
				"the %q join method requires gcp allow rules with at least one project ID",
				JoinMethodGCP,
			)
		}
	}
	return nil
}

func (a *ProvisionTokenSpecV2Spacelift) checkAndSetDefaults() error {
	if a.Hostname == "" {
		return trace.BadParameter(
			"hostname: should be set to the hostname of the spacelift tenant",
		)
	}
	if strings.Contains(a.Hostname, "/") {
		return trace.BadParameter(
			"hostname: should not contain the scheme or path",
		)
	}
	if len(a.Allow) == 0 {
		return trace.BadParameter("allow: at least one rule must be set")
	}
	for i, allowRule := range a.Allow {
		if allowRule.SpaceID == "" && allowRule.CallerID == "" {
			return trace.BadParameter(
				"allow[%d]: at least one of ['space_id', 'caller_id'] must be set",
				i,
			)
		}
	}
	return nil
}

func (a *ProvisionTokenSpecV2TPM) validate() error {
	for i, caData := range a.EKCertAllowedCAs {
		p, _ := pem.Decode([]byte(caData))
		if p == nil {
			return trace.BadParameter(
				"ekcert_allowed_cas[%d]: no pem block found",
				i,
			)
		}
		if p.Type != "CERTIFICATE" {
			return trace.BadParameter(
				"ekcert_allowed_cas[%d]: pem block is not 'CERTIFICATE' type",
				i,
			)
		}
		if _, err := x509.ParseCertificate(p.Bytes); err != nil {
			return trace.Wrap(
				err,
				"ekcert_allowed_cas[%d]: parsing certificate",
				i,
			)

		}
	}

	if len(a.Allow) == 0 {
		return trace.BadParameter(
			"allow: at least one rule must be set",
		)
	}
	for i, allowRule := range a.Allow {
		if len(allowRule.EKPublicHash) == 0 && len(allowRule.EKCertificateSerial) == 0 {
			return trace.BadParameter(
				"allow[%d]: at least one of ['ek_public_hash', 'ek_certificate_serial'] must be set",
				i,
			)
		}
	}
	return nil
}

func (a *ProvisionTokenSpecV2TerraformCloud) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one token allow rule", JoinMethodTerraformCloud)
	}

	// Note: an empty audience will fall back to the cluster name.

	for i, allowRule := range a.Allow {
		orgSet := allowRule.OrganizationID != "" || allowRule.OrganizationName != ""
		projectSet := allowRule.ProjectID != "" || allowRule.ProjectName != ""
		workspaceSet := allowRule.WorkspaceID != "" || allowRule.WorkspaceName != ""

		if !orgSet {
			return trace.BadParameter(
				"allow[%d]: one of ['organization_id', 'organization_name'] must be set",
				i,
			)
		}

		if !projectSet && !workspaceSet {
			return trace.BadParameter(
				"allow[%d]: at least one of ['project_id', 'project_name', 'workspace_id', 'workspace_name'] must be set",
				i,
			)
		}
	}

	return nil
}

func (a *ProvisionTokenSpecV2Bitbucket) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one token allow rule", JoinMethodBitbucket)
	}

	if a.Audience == "" {
		return trace.BadParameter("audience: an OpenID Connect Audience value is required")
	}

	if a.IdentityProviderURL == "" {
		return trace.BadParameter("identity_provider_url: an identity provider URL is required")
	}

	for i, rule := range a.Allow {
		workspaceSet := rule.WorkspaceUUID != ""
		repositorySet := rule.RepositoryUUID != ""

		if !workspaceSet && !repositorySet {
			return trace.BadParameter(
				"allow[%d]: at least one of ['workspace_uuid', 'repository_uuid'] must be set",
				i,
			)
		}
	}

	return nil
}

// checkAndSetDefaults checks and sets defaults on the Oracle spec. This only
// covers basics like the presence of required fields; more complex validation
// (e.g. requiring the Oracle SDK) is in auth.validateOracleJoinToken.
func (a *ProvisionTokenSpecV2Oracle) checkAndSetDefaults() error {
	if len(a.Allow) == 0 {
		return trace.BadParameter("the %q join method requires at least one allow rule", JoinMethodOracle)
	}
	for i, rule := range a.Allow {
		if rule.Tenancy == "" {
			return trace.BadParameter(
				"allow[%d]: tenancy must be set",
				i,
			)
		}
	}
	return nil
}
