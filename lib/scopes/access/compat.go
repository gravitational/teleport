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

package access

import (
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
)

// scopedRoleConditionsToRoleConditions converts scoped role conditions to classic role conditions.
func scopedRoleConditionsToRoleConditions(src *scopedaccessv1.ScopedRoleConditions) types.RoleConditions {
	var rules []types.Rule
	for _, r := range src.GetRules() {
		// as part of this conversion we expand multi-resource rules into multiple single-resource rules.
		// this is because we need to filter out unsupported resource:verb combinations, which may not be
		// sound if multiple resources are combined in a single rule.
		for _, resource := range r.GetResources() {
			var verbs []string
			for _, verb := range r.GetVerbs() {
				if !isAllowedScopedRule(resource, verb) {
					// skip verbs that are not allowed for the resource kind. note that this differs from
					// classic teleport role behavior, where we don't worry about unsupported resource:verb
					// combinations because we theoretically won't have any access checks for unsupported
					// combinations. scoped roles differ because there may be logic that is abstracting over
					// scoped and unscoped identities doing access checks for verbs that aren't supported
					// for scoped roles but *are* supported for classic roles. in such a scenario, if we
					// decide to introduce scoped support for the verb in the future, we may end up changing
					// the nature of the access check to better accommodate the scoping model. if we do,
					// outdated instances running the old access check would behave unsoundly when handling
					// the new rule.
					continue
				}
				verbs = append(verbs, verb)
			}
			if len(verbs) == 0 {
				// skip rules that have no allowed verbs.
				continue
			}
			rules = append(rules, types.Rule{
				Resources: []string{resource},
				Verbs:     verbs,
			})
		}
	}

	return types.RoleConditions{
		Rules: rules,
	}
}

// ScopedRoleToRole converts a scoped role to an equivalent classic role. Scoped roles do not implement their
// own access-control logic for the most part, and instead rely on converting to classic roles for the final
// step of evaluation. This functiona also accepts the assigned scope as a parameter because we conventionally
// format converted role names as "<role-name>@<assigned-scope>" to help ensure reasonable error messages from
// role evaluation logic.
func ScopedRoleToRole(sr *scopedaccessv1.ScopedRole, assignedScope string) (types.Role, error) {
	role, err := types.NewRoleWithVersion(sr.GetMetadata().GetName()+"@"+assignedScope, types.V8, types.RoleSpecV6{
		// scoped roles support allow blocks, but not deny blocks.
		Allow: scopedRoleConditionsToRoleConditions(sr.GetSpec().GetAllow()),
		// no scoped role options have been implemented yet. all options blocks are default/placeholder values.
		Options: types.RoleOptions{
			// CertificateFormat is always set to "standard" for scoped roles. We don't anticipate needing to change
			// this in the future, but if we did alternative options for controlling the parameter via some other
			// mechanism should be investigated. Certificate format determination via scoped roles would be problematic.
			CertificateFormat: constants.CertificateFormatStandard,
			// MaxSessionTTL must be a global default value until we decide how to handle its effect on pinned certificate
			// parameters. Likely we will need to decouple session TTL from certificate lifetime. See getScopedSessionTTL.
			MaxSessionTTL: types.NewDuration(getScopedSessionTTL()),
			// RequireMFAType is off until we decide how to handle its effect on pinned certificate parameters. This field
			// is the underlying driver of the RequiredKeyPolicy checker method/cert attr. It is enforced at cert
			// generation time, and also re-enforced during access. Due to the fact that descendant scopes must not be
			// able to interfere with access granted at ancestral scopes, we have to support usecases where a cert may or
			// may not have a given policy enforced for it depending on the scope of permit. There is a mechanism for
			// looking up attestation data in the backend, so in theory we could support lazy lookup of this value instead
			// of encoding it on certificates.  In practice, that may be sub-optimal (at least until PDP lands).
			RequireMFAType: types.RequireMFAType_OFF,
			// PinSourceIP is off until we decide how to handle its effect on pinned certificate parameters. In a perfect
			// world, we would simply always include a record of the origin address on the certificate and only enforce
			// it if the specific access decision was scoped s.t. pinning were required. In practice, this would be a very
			// different and less robust feature than the source ip pinning of classic teleport roles. In classic teleport
			// roles pinning actually sets the `source-address` critical option on the certificate, which is a well-known
			// standard and is enforced both by openssh and by the crypto/ssh package.  It is likely that no scope-aware
			// equivalent feature would ever be so robust. In light of that, it may be more desirable either to never
			// support pinning for scoped roles, or introduce a new policy type that ascribes pinning as a control
			// independent of scopes, either globally or per-user.
			PinSourceIP: false,
			// SSHPortForwarding must be a global default value until we decide how to handle its effects on pinned
			// certificate parameters. Likely the solution will be two-part. Teleport agent ssh behavior should be reviewed
			// to remove dependence on the certificate parameter in favor of always using the port forwarding permission
			// determined at the scope of permit. Certificates generated to target openssh agents should encode the parameter
			// determined at the scope of permit to the certificate.
			SSHPortForwarding: getScopedPortForwardingConfig(),
			// ForwardAgent must be a global default value until we decide how to handle its effects on pinned certificate
			// parameters. Likely will follow the same behavior as SSHPortForwarding.
			ForwardAgent: types.NewBool(getScopedForwardAgent()),
			// PermitX11Forwarding is off until we decide how to handle its effect on pinned certificate parameters. Likely
			// will follow the same behavior as SSHPortForwarding.
			PermitX11Forwarding: types.NewBool(false),
			// CertExtensions will likely *never* have a sane correlary for scoped roles. Scoped certificates include roles
			// which only apply conditionally and necessarily must not apply to all access when the certificate is pinned to
			// a parent scope. CertExtensions as a concept is incompatible with this goal.
			CertExtensions: nil,
			// Lock mode is unset (i.e. defers to cluster-wide default) until we decide how to handle its effect on pinned
			// certificate creation. Role-affected locking behavior during certificate creation doesn't map well to pinned
			// certificates. We do need to support custom locking mode for scoped roles eventually in order to make
			// per-access lock evaluation specialization possible, but its likely that cert-creation locking behavior will
			// need special handling of some kind.
			Lock: "",
		},
	})
	if err != nil {
		return nil, trace.Errorf("failed to convert scoped role %q assigned at scope %q to classic role: %v", sr.GetMetadata().GetName(), assignedScope, err)
	}

	return role, nil
}

// getScopedSessionTTL returns the session TTL for scoped access sessions. This is currently hard-coded to be 8 hours unless
// overridden by an unstable env var. We would eventually like to make this configurable, but the existing mechanics of
// session TTLs violate scope isolation principals.  We will need to do a deeper rework of the handling of session ttls and
// decouple them from certificate lifetimes before it will be sane for scoped roles to define custom session ttls. As a
// holdover, the unstable var will allow administrators some rudimentary control in the event the default is unacceptable.
// XXX: We *must not* introduce configurable session TTLs without reevaluating the behavior of the
// ScopedAccessChecker.CheckLoginDuration and ScopedAccessChecker.AdjustSessionTTL methods.
func getScopedSessionTTL() time.Duration {
	if s := os.Getenv("TELEPORT_UNSTABLE_SCOPES_SESSION_TTL"); s != "" {
		if ttl, err := time.ParseDuration(s); err == nil {
			return ttl
		}
	}

	return time.Hour * 8
}

// getScopedPortForwardingConfig returns the port forwarding configuration for scoped access. This is currently hard-coded to
// be disabled unless overridden by an unstable env var. We would eventually like to make this configurable, but
// certificate-based port forwarding permissions violate scope isolation principals. We will need to rework port forwarding
// to decouple it from certificate parameters before it will be sane for scoped roles to define custom port forwarding behavior.
// As a holdover, the unstable var will allow administrators some rudimentary control in the event the default is unacceptable.
func getScopedPortForwardingConfig() *types.SSHPortForwarding {
	value := os.Getenv("TELEPORT_UNSTABLE_SCOPES_PORT_FORWARDING") == "yes"
	return &types.SSHPortForwarding{
		Remote: &types.SSHRemotePortForwarding{Enabled: types.NewBoolOption(value)},
		Local:  &types.SSHLocalPortForwarding{Enabled: types.NewBoolOption(value)},
	}
}

// getScopedForwardAgent returns whether agent forwarding is enabled for scoped access. This is currently hard-coded to be
// disabled unless overridden by an unstable env var. We would eventually like to make this configurable, but certificate-based
// agent forwarding permissions violate scope isolation principals. We will need to rework agent forwarding to decouple it
// from certificate parameters before it will be sane for scoped roles to define custom agent forwarding behavior. As a
// holdover, the unstable var will allow administrators some rudimentary control in the event the default is unacceptable.
func getScopedForwardAgent() bool {
	return os.Getenv("TELEPORT_UNSTABLE_SCOPES_FORWARD_AGENT") == "yes"
}
