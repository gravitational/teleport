/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var resourceHandlers = map[ResourceKind]resource{
	types.KindAccessGraphSettings:                accessGraphSettings,
	types.KindCrownJewel:                         crownJewel,
	types.KindAccessList:                         accessList,
	types.KindAccessMonitoringRule:               accessMonitoringRule,
	types.KindAccessRequest:                      accessRequest,
	types.KindAppServer:                          appServer,
	types.KindApp:                                app,
	types.KindCertAuthority:                      certAuthority,
	types.KindAutoUpdateConfig:                   autoUpdateConfig,
	types.KindAutoUpdateVersion:                  autoUpdateVersion,
	types.KindAutoUpdateAgentReport:              autoUpdateAgentReport,
	types.KindAutoUpdateAgentRollout:             autoUpdateAgentRollout,
	types.KindBot:                                bot,
	types.KindBotInstance:                        botInstance,
	types.KindUIConfig:                           uiConfig,
	types.KindClusterAuthPreference:              clusterAuthPreference,
	types.KindClusterNetworkingConfig:            clusterNetworkingConfig,
	types.KindSessionRecordingConfig:             sessionRecordingConfig,
	types.KindNetworkRestrictions:                networkRestrictions,
	types.KindClusterMaintenanceConfig:           clusterMaintenanceConfig,
	types.KindConnectors:                         connector,
	types.KindOIDC:                               oidcConnector,
	types.KindSAML:                               samlConnector,
	types.KindGithub:                             githubConnector,
	types.KindDatabaseServer:                     databaseServer,
	types.KindDatabase:                           database,
	types.KindDatabaseService:                    databaseService,
	types.KindDatabaseObjectImportRule:           databaseObjectImportRule,
	types.KindDatabaseObject:                     databaseObject,
	types.KindHealthCheckConfig:                  healthCheckConfig,
	types.KindDevice:                             device,
	types.KindDiscoveryConfig:                    discoveryConfig,
	types.KindExternalAuditStorage:               externalAuditStorage,
	types.KindGitServer:                          gitServer,
	types.KindSAMLIdPServiceProvider:             samlIdPServiceProvider,
	types.KindInstaller:                          installer,
	types.KindIntegration:                        integration,
	types.KindKubeServer:                         kubeServer,
	types.KindKubernetesCluster:                  kubeCluster,
	types.KindLock:                               lock,
	types.KindLoginRule:                          loginRule,
	types.KindNamespace:                          namespace,
	types.KindOktaImportRule:                     oktaImportRule,
	types.KindOktaAssignment:                     oktaAssignment,
	types.KindUserGroup:                          userGroup,
	types.KindPlugin:                             plugin,
	types.KindReverseTunnel:                      reverseTunnel,
	types.KindRole:                               role,
	types.KindAuditQuery:                         auditQuery,
	types.KindSecurityReport:                     securityReport,
	types.KindSemaphore:                          semaphore,
	types.KindNode:                               node,
	types.KindAuthServer:                         authServer,
	types.KindProxy:                              proxy,
	types.KindServerInfo:                         serverInfo,
	types.KindStaticHostUser:                     staticHostUser,
	types.KindToken:                              token,
	types.KindTrustedCluster:                     trustedCluster,
	types.KindRemoteCluster:                      remoteCluster,
	types.KindUser:                               user,
	types.KindUserTask:                           userTask,
	types.KindVnetConfig:                         vnetConfig,
	types.KindWindowsDesktopService:              windowsDesktopService,
	types.KindWindowsDesktop:                     windowsDesktop,
	types.KindDynamicWindowsDesktop:              dynamicWindowsDesktop,
	types.KindSPIFFEFederation:                   spiffeFederation,
	types.KindWorkloadIdentity:                   workloadIdentity,
	types.KindWorkloadIdentityX509Revocation:     workloadIdentityX509Revocation,
	types.KindWorkloadIdentityX509IssuerOverride: workloadIdentityX509IssuerOverride,
	types.KindSigstorePolicy:                     sigstorePolicy,
}

type resource struct {
	getHandler    func(context.Context, *authclient.Client, services.Ref, getOpts) (collections.ResourceCollection, error)
	createHandler func(context.Context, *authclient.Client, services.UnknownResource, createOpts) error
	updateHandler func(context.Context, *authclient.Client, services.UnknownResource, createOpts) error
	deleteHandler func(context.Context, *authclient.Client, services.Ref) error
	singleton     bool
	description   string
}

type getOpts struct {
	// withSecrets is true if the user set --with-secrets
	withSecrets bool
}

type createOpts struct {
	// force is true if the user set --force
	force bool
	// confirm is true if the user set --confirm
	confirm bool
}

func (r *resource) get(ctx context.Context, clt *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if r.getHandler == nil {
		return nil, trace.NotImplemented("resource does not support 'tctl get'")
	}
	return r.getHandler(ctx, clt, ref, opts)
}

func (r *resource) create(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	if r.createHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl create'")
	}
	return r.createHandler(ctx, clt, raw, opts)
}

func (r *resource) update(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	if r.updateHandler == nil {
		return trace.NotImplemented("resource does not have an update handler")
	}
	return r.updateHandler(ctx, clt, raw, opts)
}

func (r *resource) delete(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	if ref.Name == "" && !r.singleton {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}
	if r.deleteHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl delete'")
	}
	return r.deleteHandler(ctx, clt, ref)
}

// ResourceGetHandler is the generic implementation of a resource get handler.
type ResourceGetHandler func(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error)

// ResourceCreateHandler is the generic implementation of a resource creation handler
type ResourceCreateHandler func(context.Context, *authclient.Client, services.UnknownResource) error

// ResourceDeleteHandler is the generic implementation of a resource creation handler
type ResourceDeleteHandler func(context.Context, *authclient.Client) error

// ResourceKind is the string form of a resource, i.e. "oidc"
type ResourceKind string

func NewTestResourceCommand(writer io.Writer) ResourceCommand {
	return ResourceCommand{
		stdout: writer,
	}
}

// ResourceCommand implements `tctl get/create/list` commands for manipulating
// Teleport resources
type ResourceCommand struct {
	resourceHandlers map[ResourceKind]resource
	config           *servicecfg.Config
	ref              services.Ref
	refs             services.Refs
	format           string
	namespace        string
	withSecrets      bool
	force            bool
	confirm          bool
	ttl              string
	labels           string

	// filename is the name of the resource, used for 'create'
	filename string

	// CLI subcommands:
	deleteCmd *kingpin.CmdClause
	getCmd    *kingpin.CmdClause
	createCmd *kingpin.CmdClause
	updateCmd *kingpin.CmdClause

	verbose bool

	// stdout allows to switch standard output source for resource command. Used in tests.
	stdout io.Writer
}

const getHelp = `Examples:

  $ tctl get clusters       : prints the list of all trusted clusters
  $ tctl get cluster/east   : prints the trusted cluster 'east'
  $ tctl get clusters,users : prints all trusted clusters and all users

Same as above, but using JSON output:

  $ tctl get clusters --format=json
`

// Initialize allows ResourceCommand to plug itself into the CLI parser
func (rc *ResourceCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	rc.resourceHandlers = resourceHandlers
	rc.config = config

	rc.createCmd = app.Command("create", "Create or update a Teleport resource from a YAML file.")
	rc.createCmd.Arg("filename", "resource definition file, empty for stdin").StringVar(&rc.filename)
	rc.createCmd.Flag("force", "Overwrite the resource if already exists").Short('f').BoolVar(&rc.force)
	rc.createCmd.Flag("confirm", "Confirm an unsafe or temporary resource update").Hidden().BoolVar(&rc.confirm)

	rc.updateCmd = app.Command("update", "Update resource fields.")
	rc.updateCmd.Arg("resource type/resource name", `Resource to update
	<resource type>  Type of a resource [for example: rc]
	<resource name>  Resource name to update

	Example:
	$ tctl update rc/remote`).SetValue(&rc.ref)
	rc.updateCmd.Flag("set-labels", "Set labels").StringVar(&rc.labels)
	rc.updateCmd.Flag("set-ttl", "Set TTL").StringVar(&rc.ttl)

	rc.deleteCmd = app.Command("rm", "Delete a resource.").Alias("del")
	rc.deleteCmd.Arg("resource type/resource name", `Resource to delete
	<resource type>  Type of a resource [for example: connector,user,cluster,token]
	<resource name>  Resource name to delete

	Examples:
	$ tctl rm connector/github
	$ tctl rm cluster/main`).SetValue(&rc.ref)

	rc.getCmd = app.Command("get", "Print a YAML declaration of various Teleport resources.")
	rc.getCmd.Arg("resources", "Resource spec: 'type/[name][,...]' or 'all'").Required().SetValue(&rc.refs)
	rc.getCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&rc.format)
	rc.getCmd.Flag("namespace", "Namespace of the resources").Hidden().Default(apidefaults.Namespace).StringVar(&rc.namespace)
	rc.getCmd.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&rc.withSecrets)
	rc.getCmd.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&rc.verbose)

	rc.getCmd.Alias(getHelp)

	if rc.stdout == nil {
		rc.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument (like "auth gen") and executes it
// or returns match=false if 'cmd' does not belong to it
func (rc *ResourceCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	// tctl get
	case rc.getCmd.FullCommand():
		commandFunc = rc.Get
		// tctl create
	case rc.createCmd.FullCommand():
		commandFunc = rc.Create
		// tctl rm
	case rc.deleteCmd.FullCommand():
		commandFunc = rc.Delete
		// tctl update
	case rc.updateCmd.FullCommand():
		commandFunc = rc.UpdateFields
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// IsDeleteSubcommand returns 'true' if the given command is `tctl rm`
func (rc *ResourceCommand) IsDeleteSubcommand(cmd string) bool {
	return cmd == rc.deleteCmd.FullCommand()
}

// GetRef returns the reference (basically type/name pair) of the resource
// the command is operating on
func (rc *ResourceCommand) GetRef() services.Ref {
	return rc.ref
}

// Get prints one or many resources of a certain type
func (rc *ResourceCommand) Get(ctx context.Context, client *authclient.Client) error {
	// Some resources require MFA to list with secrets. Check if we are trying to
	// get any such resources so we can prompt for MFA preemptively.
	mfaKinds := []string{types.KindToken, types.KindCertAuthority}
	mfaRequired := rc.withSecrets && slices.ContainsFunc(rc.refs, func(r services.Ref) bool {
		return slices.Contains(mfaKinds, r.Kind)
	})

	// Check if MFA has already been provided.
	if _, err := mfa.MFAResponseFromContext(ctx); err == nil {
		mfaRequired = false
	}

	if mfaRequired {
		mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
		if err == nil {
			ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
		} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
			return trace.Wrap(err)
		}
	}

	if rc.refs.IsAll() {
		return rc.GetAll(ctx, client)
	}
	if len(rc.refs) != 1 {
		return rc.GetMany(ctx, client)
	}
	rc.ref = rc.refs[0]
	collection, err := rc.getCollection(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note that only YAML is officially supported. Support for text and JSON
	// is experimental.
	switch rc.format {
	case teleport.Text:
		return collection.WriteText(rc.stdout, rc.verbose)
	case teleport.YAML:
		return collections.WriteYAML(collection, rc.stdout)
	case teleport.JSON:
		return collections.WriteJSON(collection, rc.stdout)
	}
	return trace.BadParameter("unsupported format")
}

func (rc *ResourceCommand) GetMany(ctx context.Context, client *authclient.Client) error {
	if rc.format != teleport.YAML {
		return trace.BadParameter("mixed resource types only support YAML formatting")
	}

	var resources []types.Resource
	for _, ref := range rc.refs {
		rc.ref = ref
		collection, err := rc.getCollection(ctx, client)
		if err != nil {
			return trace.Wrap(err)
		}
		resources = append(resources, collection.Resources()...)
	}
	if err := utils.WriteYAML(rc.stdout, resources); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) GetAll(ctx context.Context, client *authclient.Client) error {
	rc.withSecrets = true
	allKinds := services.GetResourceMarshalerKinds()
	allRefs := make([]services.Ref, 0, len(allKinds))
	for _, kind := range allKinds {
		ref := services.Ref{
			Kind: kind,
		}
		allRefs = append(allRefs, ref)
	}
	rc.refs = services.Refs(allRefs)
	return rc.GetMany(ctx, client)
}

// Create updates or inserts one or many resources
func (rc *ResourceCommand) Create(ctx context.Context, client *authclient.Client) (err error) {
	// Prompt for admin action MFA if required, allowing reuse for multiple resource creations.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	var reader io.Reader
	if rc.filename == "" {
		reader = os.Stdin
	} else {
		f, err := utils.OpenFileAllowingUnsafeLinks(rc.filename)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		reader = f
	}
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	count := 0
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if count == 0 {
					return trace.BadParameter("no resources found, empty input?")
				}
				return nil
			}
			return trace.Wrap(err)
		}

		// An empty document at the beginning of the input will unmarshal without error.
		// Keep reading - there may be a valid document later on.
		// https://github.com/gravitational/teleport/issues/4703
		if reflect.ValueOf(raw).IsZero() {
			continue
		}

		count++

		resourceHandler, found := rc.resourceHandlers[ResourceKind(raw.Kind)]
		if !found {
			return trace.BadParameter("resource type %q unknown, please check your tctl version", raw.Kind)
		}
		// only return in case of error, to create multiple resources
		// in case if yaml spec is a list
		opts := createOpts{
			force:   rc.force,
			confirm: rc.confirm,
		}
		if err := resourceHandler.create(ctx, client, raw, opts); err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.Wrap(err, "use -f or --force flag to overwrite")
			}
			if trace.IsNotImplemented(err) {
				return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
			}
			return trace.Wrap(err)
		}
	}
}

// Delete deletes resource by name
func (rc *ResourceCommand) Delete(ctx context.Context, client *authclient.Client) (err error) {
	resourceHandler, found := rc.resourceHandlers[ResourceKind(rc.ref.Kind)]
	if !found {
		return trace.BadParameter("resource type %q unknown, please check your tctl version", rc.ref.Kind)
	}

	if err := resourceHandler.delete(ctx, client, rc.ref); err != nil {
		if trace.IsNotImplemented(err) {
			return trace.BadParameter("deleting resources of type %q is not supported", rc.ref.Kind)
		}
		err = trace.Wrap(err, "error deleting resource %q of type %q", rc.ref.Name, rc.ref.Kind)
	}
	return nil
}

// UpdateFields updates select resource fields: expiry and labels
func (rc *ResourceCommand) UpdateFields(ctx context.Context, clt *authclient.Client) error {
	if rc.ref.Kind == "" || rc.ref.Name == "" {
		return trace.BadParameter("provide a full resource name to update, for example:\n$ tctl update rc/remote --set-labels=env=prod\n")
	}

	var err error
	var labels map[string]string
	if rc.labels != "" {
		labels, err = client.ParseLabelSpec(rc.labels)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var expiry time.Time
	if rc.ttl != "" {
		duration, err := time.ParseDuration(rc.ttl)
		if err != nil {
			return trace.Wrap(err)
		}
		expiry = time.Now().UTC().Add(duration)
	}

	if expiry.IsZero() && labels == nil {
		return trace.BadParameter("use at least one of --set-labels or --set-ttl")
	}

	switch rc.ref.Kind {
	case types.KindRemoteCluster:
		cluster, err := clt.GetRemoteCluster(ctx, rc.ref.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if labels != nil {
			meta := cluster.GetMetadata()
			meta.Labels = labels
			cluster.SetMetadata(meta)
		}
		if !expiry.IsZero() {
			cluster.SetExpiry(expiry)
		}
		if _, err = clt.UpdateRemoteCluster(ctx, cluster); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster %v has been updated\n", rc.ref.Name)
	default:
		return trace.BadParameter("updating resources of type %q is not supported, supported are: %q", rc.ref.Kind, types.KindRemoteCluster)
	}
	return nil
}

// IsForced returns true if -f flag was passed
func (rc *ResourceCommand) IsForced() bool {
	return rc.force
}

// getCollection lists all resources of a given type
func (rc *ResourceCommand) getCollection(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}

	resourceHandler, found := rc.resourceHandlers[ResourceKind(rc.ref.Kind)]
	if !found {
		return nil, trace.BadParameter("resource type %q unknown, please check your tctl version", rc.ref.Kind)
	}

	coll, err := resourceHandler.get(ctx, client, rc.ref, getOpts{withSecrets: rc.withSecrets})
	if err != nil {
		if trace.IsNotImplemented(err) {
			return nil, trace.BadParameter("getting %q is not supported", rc.ref.String())
		}
		return nil, trace.Wrap(err, "getting resource %q of type %q", rc.ref.Name, rc.ref.Kind)
	}
	return coll, nil
}

// UpsertVerb generates the correct string form of a verb based on the action taken
func UpsertVerb(exists bool, force bool) string {
	if !force && exists {
		return "updated"
	}
	return "created"
}

func checkCreateResourceWithOrigin(storedRes types.ResourceWithOrigin, resDesc string, force, confirm bool) error {
	if exists := (storedRes.Origin() != types.OriginDefaults); exists && !force {
		return trace.AlreadyExists("non-default %s already exists", resDesc)
	}
	return checkUpdateResourceWithOrigin(storedRes, resDesc, confirm)
}

func checkUpdateResourceWithOrigin(storedRes types.ResourceWithOrigin, resDesc string, confirm bool) error {
	managedByStatic := storedRes.Origin() == types.OriginConfigFile
	if managedByStatic && !confirm {
		return trace.BadParameter(`The %s resource is managed by static configuration. We recommend removing configuration from teleport.yaml, restarting the servers and trying this command again.

If you would still like to proceed, re-run the command with the --confirm flag.`, resDesc)
	}
	return nil
}

const managedByStaticDeleteMsg = `This resource is managed by static configuration. In order to reset it to defaults, remove relevant configuration from teleport.yaml and restart the servers.`

// keepFn is a predicate function that returns true if a resource should be
// retained by filterResources.
type keepFn[T types.ResourceWithLabels] func(T) bool

// filterResources takes a list of resources and returns a filtered list of
// resources for which the `keep` predicate function returns true.
func filterResources[T types.ResourceWithLabels](resources []T, keep keepFn[T]) []T {
	out := make([]T, 0, len(resources))
	for _, r := range resources {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

// altNameFn is a func that returns an alternative name for a resource.
type altNameFn[T types.ResourceWithLabels] func(T) string

// filterByNameOrDiscoveredName filters resources by name or "discovered name".
// It prefers exact name filtering first - if none of the resource names match
// exactly (i.e. all of the resources are filtered out), then it retries and
// filters the resources by "discovered name" of resource name instead, which
// comes from an auto-discovery label.
func filterByNameOrDiscoveredName[T types.ResourceWithLabels](resources []T, prefixOrName string, extra ...altNameFn[T]) []T {
	// prefer exact names
	out := filterByName(resources, prefixOrName, extra...)
	if len(out) == 0 {
		// fallback to looking for discovered name label matches.
		out = filterByDiscoveredName(resources, prefixOrName)
	}
	return out
}

// filterByName filters resources by exact name match.
func filterByName[T types.ResourceWithLabels](resources []T, name string, altNameFns ...altNameFn[T]) []T {
	return filterResources(resources, func(r T) bool {
		if r.GetName() == name {
			return true
		}
		for _, altName := range altNameFns {
			if altName(r) == name {
				return true
			}
		}
		return false
	})
}

// filterByDiscoveredName filters resources that have a "discovered name" label
// that matches the given name.
func filterByDiscoveredName[T types.ResourceWithLabels](resources []T, name string) []T {
	return filterResources(resources, func(r T) bool {
		discoveredName, ok := r.GetLabel(types.DiscoveredNameLabel)
		return ok && discoveredName == name
	})
}

// getOneResourceNameToDelete checks a list of resources to ensure there is
// exactly one resource name among them, and returns that name or an error.
// Heartbeat resources can have the same name but different host ID, so this
// still allows a user to delete multiple heartbeats of the same name, for
// example `$ tctl rm db_server/someDB`.
func getOneResourceNameToDelete[T types.ResourceWithLabels](rs []T, ref services.Ref, resDesc string) (string, error) {
	seen := make(map[string]struct{})
	for _, r := range rs {
		seen[r.GetName()] = struct{}{}
	}
	switch len(seen) {
	case 1: // need exactly one.
		return rs[0].GetName(), nil
	case 0:
		return "", trace.NotFound("%v %q not found", resDesc, ref.Name)
	default:
		names := make([]string, 0, len(rs))
		for _, r := range rs {
			names = append(names, r.GetName())
		}
		msg := formatAmbiguousDeleteMessage(ref, resDesc, names)
		return "", trace.BadParameter("%s", msg)
	}
}

// formatAmbiguousDeleteMessage returns a formatted message when a user is
// attempting to delete multiple resources by an ambiguous prefix of the
// resource names.
func formatAmbiguousDeleteMessage(ref services.Ref, resDesc string, names []string) string {
	slices.Sort(names)
	// choose an actual resource for the example in the error.
	exampleRef := ref
	exampleRef.Name = names[0]
	return fmt.Sprintf(`%s matches multiple auto-discovered %vs:
%v

Use the full resource name that was generated by the Teleport Discovery service, for example:
$ tctl rm %s`,
		ref.String(), resDesc, strings.Join(names, "\n"), exampleRef.String())
}
