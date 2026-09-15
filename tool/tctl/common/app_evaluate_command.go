/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/appresource"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const evaluateRequestHelpText = `Evaluate one HTTP request against the app_resources
and app_resources_expressions rules of v9 or newer roles and print the decision.

URL is the app URL, the request path included. The request is evaluated
against the cluster for the logged-in user.`

const evaluateResourceHelpText = `Evaluate one request for an app resource against
the app_resources and app_resources_expressions rules of v9 or newer roles and
print the decision.

PATH is the path an app resource is reached at, such as /api/v4/projects. The
roles come from the --spec file and no cluster is contacted. Without a user
resource in the file, every role in it is evaluated. The file holds no app, so
app_labels are ignored and every role counts as granting the app.`

// appsEvaluateFlags is the part of an evaluate subcommand that both
// subcommands share.
type appsEvaluateFlags struct {
	command *kingpin.CmdClause

	method string
	format string

	// stdout and stderr are the decision and warning streams. Unset, they
	// default to os.Stdout and os.Stderr.
	stdout io.Writer
	stderr io.Writer
}

// addCommonFlags stores command, defaults the output streams, and registers
// the flags both evaluate subcommands take.
func (c *appsEvaluateFlags) addCommonFlags(command *kingpin.CmdClause) {
	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	if c.stderr == nil {
		c.stderr = os.Stderr
	}
	c.command = command
	command.Flag("method", "Request method, in upper or lower case.").Default("GET").StringVar(&c.method)
	command.Flag("format", "Output format.").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON)
}

// FullCommand returns the fully qualified name of the subcommand.
func (c *appsEvaluateFlags) FullCommand() string {
	return c.command.FullCommand()
}

// appsEvaluateRequestCommand implements "tctl apps evaluate-request".
type appsEvaluateRequestCommand struct {
	appsEvaluateFlags

	url string
}

// Initialize registers the subcommand, its argument, and its flags under apps.
func (c *appsEvaluateRequestCommand) Initialize(apps *kingpin.CmdClause) {
	command := apps.Command("evaluate-request", evaluateRequestHelpText)
	c.addCommonFlags(command)
	command.Arg("url", "App URL, request path included.").Required().StringVar(&c.url)
}

// run evaluates the request against the roles of the logged-in user
// that grant the app at URL, and writes the decision to stdout. Roles are
// matched to the app with the user's traits applied. A rule under deny in
// any of the user's roles denies.
func (c *appsEvaluateRequestCommand) run(ctx context.Context, clt *authclient.Client) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return trace.Wrap(err, "parsing URL")
	}
	if u.Scheme == "" || u.Hostname() == "" {
		return trace.BadParameter("URL must be absolute and have a host, such as https://app.example.com/path")
	}
	app, err := appByHost(ctx, c.stderr, clt, u.Hostname())
	if err != nil {
		return trace.Wrap(err)
	}
	if !appresource.GovernsApp(app) {
		return trace.BadParameter("app_resources rules do not govern app %q", app.GetName())
	}
	const hint = "evaluate-request reads the roles of the logged-in user. A host identity has none. Log in with tsh, or use evaluate-resource with --spec."
	user, err := clt.GetCurrentUser(ctx)
	if err != nil {
		return trace.Wrap(err, hint)
	}
	roles, err := clt.GetCurrentUserRoles(ctx)
	if err != nil {
		return trace.Wrap(err, hint)
	}
	identity := identityFromUser(user)
	granting, err := rolesGrantingApp(roles, app, identity)
	if err != nil {
		return trace.Wrap(err)
	}
	// Scan every held role, because a deny rule's scope is unknown. A deny
	// applies only where a v9 or newer role grants the app.
	if appresource.PartitionRoles(granting).Enforced() {
		if decision, ok := denyRulesDecision(c.stderr, roles); ok {
			return trace.Wrap(writeDecision(c.stdout, decision, c.format))
		}
	}
	request := appresource.Request{
		Method: strings.ToUpper(c.method),
		Path:   cmp.Or(u.EscapedPath(), "/"),
	}
	decision, err := evaluateRoles(c.stderr, granting, request, identity)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(writeDecision(c.stdout, decision, c.format))
}

// appsEvaluateResourceCommand implements "tctl apps evaluate-resource".
type appsEvaluateResourceCommand struct {
	appsEvaluateFlags

	path string
	spec string
}

// Initialize registers the subcommand, its argument, and its flags under apps.
func (c *appsEvaluateResourceCommand) Initialize(apps *kingpin.CmdClause) {
	command := apps.Command("evaluate-resource", evaluateResourceHelpText)
	c.addCommonFlags(command)
	command.Arg("path", "Request path, such as /api/v4/projects.").Required().StringVar(&c.path)
	command.Flag("spec", "YAML file with one or more role resources and an optional user resource. The request is evaluated against the user's roles without contacting a cluster, or against every role in the file when there is no user resource.").Required().StringVar(&c.spec)
}

// run evaluates the request against the roles in the --spec file and writes
// the decision to stdout. The user resource, if present, is the identity the
// request is evaluated for and selects the roles.
func (c *appsEvaluateResourceCommand) run() error {
	spec, err := c.readSpec()
	if err != nil {
		return trace.Wrap(err)
	}
	roles := spec.roles
	var identity appresource.Identity
	if spec.user != nil {
		identity = identityFromUser(spec.user)
		roles, err = filterRolesByName(roles, identity.Roles)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// Every role in the file is evaluated, so user.roles lists them all.
		identity.Roles = sortedNames(roles)
	}
	if names := rolesWithAppLabels(roles); len(names) > 0 {
		fmt.Fprintf(c.stderr, "Warning: roles %s set app_labels, which are ignored because the spec holds no app\n", quotedList(names))
	}
	if decision, ok := denyRulesDecision(c.stderr, roles); ok {
		return trace.Wrap(writeDecision(c.stdout, decision, c.format))
	}
	// Parse the path so a space is escaped as it would be inside a URL.
	u, err := url.Parse(c.path)
	if err != nil {
		return trace.Wrap(err, "parsing PATH")
	}
	if u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" {
		return trace.BadParameter("PATH %q must be a path only, such as /api/v4/projects", c.path)
	}
	request := appresource.Request{Method: strings.ToUpper(c.method), Path: u.EscapedPath()}
	decision, err := evaluateRoles(c.stderr, roles, request, identity)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(writeDecision(c.stdout, decision, c.format))
}

// rolesWithAppLabels returns the names of the roles that set app_labels under
// allow or deny, sorted.
func rolesWithAppLabels(roles []types.Role) []string {
	var names []string
	for _, role := range roles {
		if len(role.GetAppLabels(types.Allow)) > 0 || len(role.GetAppLabels(types.Deny)) > 0 {
			names = append(names, role.GetName())
		}
	}
	slices.Sort(names)
	return names
}

// filterRolesByName returns the roles whose name is in names, in the order of
// roles. It returns an error for a name with no role.
func filterRolesByName(roles []types.Role, names []string) ([]types.Role, error) {
	var kept []types.Role
	for _, role := range roles {
		if slices.Contains(names, role.GetName()) {
			kept = append(kept, role)
		}
	}
	for _, name := range names {
		if !slices.ContainsFunc(roles, func(role types.Role) bool { return role.GetName() == name }) {
			return nil, trace.BadParameter("the spec contains no role %q, which the user holds", name)
		}
	}
	return kept, nil
}

// identityFromUser returns the name, roles, and traits of user.
func identityFromUser(user types.User) appresource.Identity {
	return appresource.Identity{
		Name:   user.GetName(),
		Roles:  user.GetRoles(),
		Traits: user.GetTraits(),
	}
}

// evaluateRoles returns the [appresource.Decision] that allows or denies
// request under the app_resources rules of roles. Warnings go to stderr.
func evaluateRoles(stderr io.Writer, roles []types.Role, request appresource.Request, identity appresource.Identity) (appresource.Decision, error) {
	if len(roles) == 0 {
		fmt.Fprintln(stderr, "Warning: no role to evaluate")
	}
	partition := appresource.PartitionRoles(roles)
	for _, err := range partition.Errors {
		fmt.Fprintf(stderr, "Warning: %s, so the role is not evaluated\n", shortUserMessage(err))
	}
	if !partition.Enforced() && len(partition.IgnoredRoleNames) > 0 {
		fmt.Fprintln(stderr, "Warning: every role predates v9, so app_resources rules do not apply")
		return appresource.Decision{Allowed: true, Roles: partition.IgnoredRoleNames}, nil
	}
	if len(partition.IgnoredRoleNames) > 0 {
		fmt.Fprintf(stderr, "Warning: dropped pre-v9 roles %s, because a v9 role is present\n", quotedList(partition.IgnoredRoleNames))
	}
	// A version-skew deny lists every role of v9 or newer, the unreadable ones
	// included.
	skewNames := make([]string, 0, len(partition.Roles)+len(partition.Errors))
	for _, role := range partition.Roles {
		skewNames = append(skewNames, role.Name)
	}
	for _, err := range partition.Errors {
		skewNames = append(skewNames, err.Name)
	}
	// A role whose rule does not compile is dropped like a role that cannot
	// be read, so an allow_all rule of another role still allows.
	unreadable := len(partition.Errors) > 0
	var compilable []appresource.Role
	for _, role := range partition.Roles {
		if _, err := appresource.CompileRoles([]appresource.Role{role}); err != nil {
			fmt.Fprintf(stderr, "Warning: %s, so the role is not evaluated\n", shortUserMessage(err))
			unreadable = true
			continue
		}
		compilable = append(compilable, role)
	}
	compiledRoles, err := appresource.CompileRoles(compilable)
	if err != nil {
		return appresource.Decision{}, trace.Wrap(err)
	}
	decision, err := compiledRoles.Evaluate(request, identity)
	if err != nil {
		fmt.Fprintf(stderr, "Warning: %s, so the request is denied\n", shortUserMessage(err))
		return denyDecision(appresource.DenyRoleVersionUnsupported, skewNames), nil
	}
	if unreadable && !allowedByAllowAll(decision, compilable) {
		return denyDecision(appresource.DenyRoleVersionUnsupported, skewNames), nil
	}
	return decision, nil
}

// quotedList returns items quoted and separated by ", ".
func quotedList(items []string) string {
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", item)
	}
	return b.String()
}

// denyRulesDecision returns a deny decision and true when a role has rules
// under deny. It returns false when no role has one.
func denyRulesDecision(stderr io.Writer, roles []types.Role) (appresource.Decision, bool) {
	denying, ok := appresource.HasDenyRules(roles)
	if !ok {
		return appresource.Decision{}, false
	}
	fmt.Fprintf(stderr, "Warning: role %q sets app_resources or app_resources_expressions under deny, so the request is denied\n", denying)
	return appresource.Decision{
		Deny:  &appresource.DenyDetails{Kind: appresource.DenyRoleVersionUnsupported},
		Roles: sortedNames(roles),
	}, true
}

// sortedNames returns the names of items, sorted.
func sortedNames[T interface{ GetName() string }](items []T) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.GetName())
	}
	slices.Sort(names)
	return names
}

// rolesGrantingApp returns the roles whose app_labels match app, with the
// identity's traits applied.
func rolesGrantingApp(roles []types.Role, app types.Application, identity appresource.Identity) ([]types.Role, error) {
	var granting []types.Role
	for _, role := range roles {
		templateCtx := services.RoleTemplateContext{Username: identity.Name, Traits: identity.Traits}
		role, err := services.ApplyTraitsWithContext(role, templateCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if services.RoleGrantsResource(role, app, identity.Name, identity.Traits) {
			granting = append(granting, role)
		}
	}
	return granting, nil
}

// shortUserMessage returns the outermost message of a wrapped error.
func shortUserMessage(err error) string {
	first, _, _ := strings.Cut(trace.UserMessage(err), "\n\t")
	return first
}

// allowedByAllowAll returns true when decision allows the request through a
// role with an allow_all rule.
func allowedByAllowAll(decision appresource.Decision, roles []appresource.Role) bool {
	if !decision.Allowed || decision.Allow == nil {
		return false
	}
	for _, role := range roles {
		if role.Name == decision.Allow.Role && role.HasAllowAll() {
			return true
		}
	}
	return false
}

// denyDecision returns a deny of kind. The returned Roles field lists names,
// sorted.
func denyDecision(kind appresource.DenyKind, names []string) appresource.Decision {
	names = slices.Clone(names)
	slices.Sort(names)
	return appresource.Decision{
		Deny:  &appresource.DenyDetails{Kind: kind},
		Roles: names,
	}
}

// spec is the content of a --spec file, role resources and an optional user
// resource as YAML documents separated by ---. user is nil when the file has
// no user resource.
type spec struct {
	roles []types.Role
	user  types.User
}

// readSpec returns the content of the file at --spec.
func (c *appsEvaluateResourceCommand) readSpec() (spec, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(c.spec)
	if err != nil {
		return spec{}, trace.Wrap(err)
	}
	defer f.Close()
	spec, err := parseSpec(f)
	return spec, trace.Wrap(err)
}

// parseSpec returns the role resources and the optional user resource read
// from r. It returns an error for a resource of any other kind.
func parseSpec(r io.Reader) (spec, error) {
	var s spec
	decoder := kyaml.NewYAMLOrJSONDecoder(r, defaults.LookaheadBufSize)
	for doc := 1; ; doc++ {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return spec{}, trace.Wrap(err)
		}
		if reflect.ValueOf(raw).IsZero() {
			// A document with only comments decodes to the zero value.
			continue
		}
		switch raw.Kind {
		case types.KindRole:
			role, err := services.UnmarshalRole(raw.Raw)
			if err != nil {
				return spec{}, trace.Wrap(err)
			}
			if slices.ContainsFunc(s.roles, func(r types.Role) bool { return r.GetName() == role.GetName() }) {
				return spec{}, trace.BadParameter("the spec contains more than one role named %q", role.GetName())
			}
			s.roles = append(s.roles, role)
		case types.KindUser:
			if s.user != nil {
				return spec{}, trace.BadParameter("the spec contains more than one user")
			}
			s.user, err = services.UnmarshalUser(raw.Raw)
			if err != nil {
				return spec{}, trace.Wrap(err)
			}
		case "":
			return spec{}, trace.BadParameter("the spec document %d has no kind", doc)
		default:
			return spec{}, trace.BadParameter("the spec document %d has kind %q, only role and user documents are read", doc, raw.Kind)
		}
	}
	if len(s.roles) == 0 {
		return spec{}, trace.BadParameter("the spec contains no role")
	}
	return s, nil
}

// appByHost returns the registered app whose public address is host. When
// none matches, host must be <app name>.<proxy public address>, and the app
// with that name matches. When apps with two different names share the
// public address, appByHost returns the first by name and writes a warning
// to stderr.
func appByHost(ctx context.Context, stderr io.Writer, clt *authclient.Client, host string) (types.Application, error) {
	servers, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var matched []types.Application
	for _, server := range servers {
		app := server.GetApp()
		if app.GetPublicAddr() != host {
			continue
		}
		if slices.ContainsFunc(matched, func(a types.Application) bool { return a.GetName() == app.GetName() }) {
			// The same app, registered by another app server.
			continue
		}
		matched = append(matched, app)
	}
	slices.SortFunc(matched, func(a, b types.Application) int { return strings.Compare(a.GetName(), b.GetName()) })
	if len(matched) > 1 {
		fmt.Fprintf(stderr, "Warning: apps %s share host %q, so %q is evaluated\n", quotedList(sortedNames(matched)), host, matched[0].GetName())
	}
	if len(matched) > 0 {
		return matched[0], nil
	}
	// An app with no public_addr is reachable at <app name>.<proxy public
	// addr>. Stripping the proxy suffix leaves the app name, dots included, so
	// foo.bar.example.com resolves to the app named foo.bar, not foo.
	proxyHost, err := proxyPublicHost(ctx, clt, host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	name := strings.TrimSuffix(host, "."+proxyHost)
	for _, server := range servers {
		if app := server.GetApp(); app.GetName() == name {
			return app, nil
		}
	}
	return nil, trace.NotFound("no app matches host %q. It must be registered, granted by your roles, and readable as app_server.", host)
}

// proxyPublicHost returns the proxy public address, without its port, that
// host is a subdomain of. It returns an error when host is a subdomain of no
// proxy public address.
func proxyPublicHost(ctx context.Context, clt *authclient.Client, host string) (string, error) {
	var proxyHosts []string
	needClusterName := false
	for proxy, err := range clientutils.Resources(ctx, clt.ListProxyServers) {
		if err != nil {
			return "", trace.Wrap(err)
		}
		named := false
		for _, addr := range proxy.GetPublicAddrs() {
			// Build the list the proxy builds in its own proxyDNSNames, which
			// drops the port and skips an address that is an IP.
			proxyHost, err := utils.DNSName(addr)
			if err != nil {
				continue
			}
			proxyHosts = append(proxyHosts, proxyHost)
			named = true
		}
		// Each proxy runs proxyDNSNames over its own public addresses, so one
		// named proxy does not cover a proxy that has no name of its own.
		if !named {
			needClusterName = true
		}
	}
	if needClusterName || len(proxyHosts) == 0 {
		// A proxy with no name of its own serves apps under the cluster name.
		clusterName, err := clt.GetClusterName(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		proxyHosts = append(proxyHosts, clusterName.GetClusterName())
	}
	// FindMatchingProxyDNS tries the longest suffix of host first, as the
	// proxy does, and falls back to the first name when none matches.
	proxyHost := utils.FindMatchingProxyDNS(host, proxyHosts)
	if proxyHost == "" || !strings.HasSuffix(host, "."+proxyHost) {
		return "", trace.BadParameter("host %q is not an app public address or a subdomain of a proxy public address", host)
	}
	return proxyHost, nil
}

// writeDecision writes decision to w as YAML or JSON.
func writeDecision(w io.Writer, decision appresource.Decision, format string) error {
	switch format {
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, decision))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(w, decision))
	default:
		return trace.BadParameter("unknown format %q, use yaml or json", format)
	}
}
