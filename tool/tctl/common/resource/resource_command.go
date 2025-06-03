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
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
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

// ResourceGetHandler is the generic implementation of a resource get handler.
type ResourceGetHandler func(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error)

// ResourceCreateHandler is the generic implementation of a resource creation handler
type ResourceCreateHandler func(context.Context, *authclient.Client, services.UnknownResource) error

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
	config      *servicecfg.Config
	ref         services.Ref
	refs        services.Refs
	format      string
	namespace   string
	withSecrets bool
	force       bool
	confirm     bool
	ttl         string
	labels      string

	// filename is the name of the resource, used for 'create'
	filename string

	// CLI subcommands:
	deleteCmd *kingpin.CmdClause
	getCmd    *kingpin.CmdClause
	createCmd *kingpin.CmdClause
	updateCmd *kingpin.CmdClause

	verbose bool

	GetHandlers    map[ResourceKind]ResourceGetHandler
	CreateHandlers map[ResourceKind]ResourceCreateHandler
	UpdateHandlers map[ResourceKind]ResourceCreateHandler

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
	rc.GetHandlers = map[ResourceKind]ResourceGetHandler{
		types.KindUser:                               rc.getUser,
		types.KindConnectors:                         rc.getConnectors,
		types.KindSAMLConnector:                      rc.getSAMLConnectors,
		types.KindOIDCConnector:                      rc.getOIDCConnectors,
		types.KindGithubConnector:                    rc.getGithubConnectors,
		types.KindReverseTunnel:                      rc.getReverseTunnel,
		types.KindCertAuthority:                      rc.getCertAuthority,
		types.KindNode:                               rc.getNode,
		types.KindAuthServer:                         rc.getAuthServer,
		types.KindProxy:                              rc.getProxy,
		types.KindRole:                               rc.getRole,
		types.KindNamespace:                          rc.getNamespace,
		types.KindTrustedCluster:                     rc.getTrustedCluster,
		types.KindRemoteCluster:                      rc.getRemoteCluster,
		types.KindSemaphore:                          rc.getSemaphore,
		types.KindClusterAuthPreference:              rc.getAuthPreference,
		types.KindClusterNetworkingConfig:            rc.getClusterNetworkingConfig,
		types.KindClusterMaintenanceConfig:           rc.getClusterMaintenanceConfig,
		types.KindSessionRecordingConfig:             rc.getSessionRecordingConfig,
		types.KindLock:                               rc.getLock,
		types.KindDatabaseServer:                     rc.getDatabaseServer,
		types.KindKubeServer:                         rc.getKubeServer,
		types.KindAppServer:                          rc.getAppServer,
		types.KindNetworkRestrictions:                rc.getNetworkRestrictions,
		types.KindApp:                                rc.getApp,
		types.KindDatabase:                           rc.getDatabase,
		types.KindKubernetesCluster:                  rc.getKubeCluster,
		types.KindCrownJewel:                         rc.getCrownJewel,
		types.KindWindowsDesktopService:              rc.getWindowsDesktopService,
		types.KindWindowsDesktop:                     rc.getWindowsDesktop,
		types.KindDynamicWindowsDesktop:              rc.getDynamicWindowsDesktop,
		types.KindToken:                              rc.getToken,
		types.KindInstaller:                          rc.getInstaller,
		types.KindUIConfig:                           rc.getUIConfig,
		types.KindDatabaseService:                    rc.getDatabaseService,
		types.KindLoginRule:                          rc.getLoginRule,
		types.KindSAMLIdPServiceProvider:             rc.getSAMLIdPServiceProvider,
		types.KindDevice:                             rc.getDevice,
		types.KindBot:                                rc.getBot,
		types.KindDatabaseObjectImportRule:           rc.getDatabaseObjectImportRule,
		types.KindDatabaseObject:                     rc.getDatabaseObject,
		types.KindOktaImportRule:                     rc.getOktaImportRule,
		types.KindOktaAssignment:                     rc.getOktaAssignment,
		types.KindUserGroup:                          rc.getUserGroup,
		types.KindExternalAuditStorage:               rc.getExternalAuditStorage,
		types.KindIntegration:                        rc.getIntegration,
		types.KindUserTask:                           rc.getUserTask,
		types.KindDiscoveryConfig:                    rc.getDiscoveryConfig,
		types.KindAuditQuery:                         rc.getAuditQuery,
		types.KindSecurityReport:                     rc.getSecurityReport,
		types.KindServerInfo:                         rc.getServerInfo,
		types.KindAccessList:                         rc.getAccessList,
		types.KindVnetConfig:                         rc.getVnetConfig,
		types.KindAccessRequest:                      rc.getAccessRequest,
		types.KindPlugin:                             rc.getPlugin,
		types.KindAccessGraphSettings:                rc.getAccessGraphSettings,
		types.KindSPIFFEFederation:                   rc.getSPIFFEFederation,
		types.KindWorkloadIdentity:                   rc.getWorkloadIdentity,
		types.KindWorkloadIdentityX509Revocation:     rc.getWorkloadIdentityX509Revocation,
		types.KindBotInstance:                        rc.getBotInstance,
		types.KindStaticHostUser:                     rc.getStaticHostUser,
		types.KindAutoUpdateConfig:                   rc.getAutoUpdateConfig,
		types.KindAutoUpdateVersion:                  rc.getAutoUpdateVersion,
		types.KindAutoUpdateAgentRollout:             rc.getAutoUpdateAgentRollout,
		types.KindAutoUpdateAgentReport:              rc.getAutoUpdateAgentReport,
		types.KindAccessMonitoringRule:               rc.getAccessMonitoringRule,
		types.KindGitServer:                          rc.getGitServer,
		types.KindWorkloadIdentityX509IssuerOverride: rc.getWorkloadIdentityX509IssuerOverride,
		types.KindSigstorePolicy:                     rc.getSigstorePolicy,
		types.KindHealthCheckConfig:                  rc.getHealthCheckConfig,
	}
	rc.CreateHandlers = map[ResourceKind]ResourceCreateHandler{
		types.KindUser:                               rc.createUser,
		types.KindRole:                               rc.createRole,
		types.KindTrustedCluster:                     rc.createTrustedCluster,
		types.KindGithubConnector:                    rc.createGithubConnector,
		types.KindCertAuthority:                      rc.createCertAuthority,
		types.KindClusterAuthPreference:              rc.createAuthPreference,
		types.KindClusterNetworkingConfig:            rc.createClusterNetworkingConfig,
		types.KindClusterMaintenanceConfig:           rc.createClusterMaintenanceConfig,
		types.KindSessionRecordingConfig:             rc.createSessionRecordingConfig,
		types.KindExternalAuditStorage:               rc.createExternalAuditStorage,
		types.KindUIConfig:                           rc.createUIConfig,
		types.KindLock:                               rc.createLock,
		types.KindNetworkRestrictions:                rc.createNetworkRestrictions,
		types.KindApp:                                rc.createApp,
		types.KindAppServer:                          rc.createAppServer,
		types.KindDatabase:                           rc.createDatabase,
		types.KindKubernetesCluster:                  rc.createKubeCluster,
		types.KindToken:                              rc.createToken,
		types.KindInstaller:                          rc.createInstaller,
		types.KindNode:                               rc.createNode,
		types.KindOIDCConnector:                      rc.createOIDCConnector,
		types.KindSAMLConnector:                      rc.createSAMLConnector,
		types.KindLoginRule:                          rc.createLoginRule,
		types.KindSAMLIdPServiceProvider:             rc.createSAMLIdPServiceProvider,
		types.KindDevice:                             rc.createDevice,
		types.KindOktaImportRule:                     rc.createOktaImportRule,
		types.KindIntegration:                        rc.createIntegration,
		types.KindWindowsDesktop:                     rc.createWindowsDesktop,
		types.KindDynamicWindowsDesktop:              rc.createDynamicWindowsDesktop,
		types.KindAccessList:                         rc.createAccessList,
		types.KindDiscoveryConfig:                    rc.createDiscoveryConfig,
		types.KindAuditQuery:                         rc.createAuditQuery,
		types.KindSecurityReport:                     rc.createSecurityReport,
		types.KindServerInfo:                         rc.createServerInfo,
		types.KindBot:                                rc.createBot,
		types.KindDatabaseObjectImportRule:           rc.createDatabaseObjectImportRule,
		types.KindDatabaseObject:                     rc.createDatabaseObject,
		types.KindAccessMonitoringRule:               rc.createAccessMonitoringRule,
		types.KindCrownJewel:                         rc.createCrownJewel,
		types.KindVnetConfig:                         rc.createVnetConfig,
		types.KindAccessGraphSettings:                rc.upsertAccessGraphSettings,
		types.KindPlugin:                             rc.createPlugin,
		types.KindSPIFFEFederation:                   rc.createSPIFFEFederation,
		types.KindWorkloadIdentity:                   rc.createWorkloadIdentity,
		types.KindStaticHostUser:                     rc.createStaticHostUser,
		types.KindUserTask:                           rc.createUserTask,
		types.KindAutoUpdateConfig:                   rc.createAutoUpdateConfig,
		types.KindAutoUpdateVersion:                  rc.createAutoUpdateVersion,
		types.KindGitServer:                          rc.createGitServer,
		types.KindAutoUpdateAgentRollout:             rc.createAutoUpdateAgentRollout,
		types.KindAutoUpdateAgentReport:              rc.upsertAutoUpdateAgentReport,
		types.KindWorkloadIdentityX509IssuerOverride: rc.createWorkloadIdentityX509IssuerOverride,
		types.KindSigstorePolicy:                     rc.createSigstorePolicy,
		types.KindHealthCheckConfig:                  rc.createHealthCheckConfig,
	}
	rc.UpdateHandlers = map[ResourceKind]ResourceCreateHandler{
		types.KindUser:                               rc.updateUser,
		types.KindGithubConnector:                    rc.updateGithubConnector,
		types.KindOIDCConnector:                      rc.updateOIDCConnector,
		types.KindSAMLConnector:                      rc.updateSAMLConnector,
		types.KindRole:                               rc.updateRole,
		types.KindClusterNetworkingConfig:            rc.updateClusterNetworkingConfig,
		types.KindClusterAuthPreference:              rc.updateAuthPreference,
		types.KindSessionRecordingConfig:             rc.updateSessionRecordingConfig,
		types.KindAccessMonitoringRule:               rc.updateAccessMonitoringRule,
		types.KindCrownJewel:                         rc.updateCrownJewel,
		types.KindVnetConfig:                         rc.updateVnetConfig,
		types.KindAccessGraphSettings:                rc.updateAccessGraphSettings,
		types.KindPlugin:                             rc.updatePlugin,
		types.KindStaticHostUser:                     rc.updateStaticHostUser,
		types.KindUserTask:                           rc.updateUserTask,
		types.KindAutoUpdateConfig:                   rc.updateAutoUpdateConfig,
		types.KindAutoUpdateVersion:                  rc.updateAutoUpdateVersion,
		types.KindDynamicWindowsDesktop:              rc.updateDynamicWindowsDesktop,
		types.KindGitServer:                          rc.updateGitServer,
		types.KindAutoUpdateAgentRollout:             rc.updateAutoUpdateAgentRollout,
		types.KindAutoUpdateAgentReport:              rc.upsertAutoUpdateAgentReport,
		types.KindWorkloadIdentityX509IssuerOverride: rc.updateWorkloadIdentityX509IssuerOverride,
		types.KindSigstorePolicy:                     rc.updateSigstorePolicy,
		types.KindHealthCheckConfig:                  rc.updateHealthCheckConfig,
	}
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

		// locate the creator function for a given resource kind:
		creator, found := rc.CreateHandlers[ResourceKind(raw.Kind)]
		if !found {
			return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
		}
		// only return in case of error, to create multiple resources
		// in case if yaml spec is a list
		if err := creator(ctx, client, raw); err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.Wrap(err, "use -f or --force flag to overwrite")
			}
			return trace.Wrap(err)
		}
	}
}

// Delete deletes resource by name
func (rc *ResourceCommand) Delete(ctx context.Context, client *authclient.Client) (err error) {
	singletonResources := []string{
		types.KindClusterAuthPreference,
		types.KindClusterMaintenanceConfig,
		types.KindClusterNetworkingConfig,
		types.KindSessionRecordingConfig,
		types.KindInstaller,
		types.KindUIConfig,
		types.KindNetworkRestrictions,
		types.KindAutoUpdateConfig,
		types.KindAutoUpdateVersion,
		types.KindAutoUpdateAgentRollout,
	}
	if !slices.Contains(singletonResources, rc.ref.Kind) && (rc.ref.Kind == "" || rc.ref.Name == "") {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}

	switch rc.ref.Kind {
	case types.KindNode:
		if err = client.DeleteNode(ctx, apidefaults.Namespace, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("node %v has been deleted\n", rc.ref.Name)
	case types.KindUser:
		if err = client.DeleteUser(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been deleted\n", rc.ref.Name)
	case types.KindRole:
		if err = client.DeleteRole(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("role %q has been deleted\n", rc.ref.Name)
	case types.KindToken:
		if err = client.DeleteToken(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("token %q has been deleted\n", rc.ref.Name)
	case types.KindSAMLConnector:
		if err = client.DeleteSAMLConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML connector %v has been deleted\n", rc.ref.Name)
	case types.KindOIDCConnector:
		if err = client.DeleteOIDCConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("OIDC connector %v has been deleted\n", rc.ref.Name)
	case types.KindGithubConnector:
		if err = client.DeleteGithubConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("github connector %q has been deleted\n", rc.ref.Name)
	case types.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", rc.ref.Name)
	case types.KindTrustedCluster:
		if err = client.DeleteTrustedCluster(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", rc.ref.Name)
	case types.KindRemoteCluster:
		if err = client.DeleteRemoteCluster(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("remote cluster %q has been deleted\n", rc.ref.Name)
	case types.KindSemaphore:
		if rc.ref.SubKind == "" || rc.ref.Name == "" {
			return trace.BadParameter(
				"full semaphore path must be specified (e.g. '%s/%s/alice@example.com')",
				types.KindSemaphore, types.SemaphoreKindConnection,
			)
		}
		err := client.DeleteSemaphore(ctx, types.SemaphoreFilter{
			SemaphoreKind: rc.ref.SubKind,
			SemaphoreName: rc.ref.Name,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("semaphore '%s/%s' has been deleted\n", rc.ref.SubKind, rc.ref.Name)
	case types.KindClusterAuthPreference:
		if err = resetAuthPreference(ctx, client); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster auth preference has been reset to defaults\n")
	case types.KindClusterMaintenanceConfig:
		if err := client.DeleteClusterMaintenanceConfig(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster maintenance configuration has been deleted\n")
	case types.KindClusterNetworkingConfig:
		if err = resetClusterNetworkingConfig(ctx, client); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster networking configuration has been reset to defaults\n")
	case types.KindSessionRecordingConfig:
		if err = resetSessionRecordingConfig(ctx, client); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("session recording configuration has been reset to defaults\n")
	case types.KindExternalAuditStorage:
		if rc.ref.Name == types.MetaNameExternalAuditStorageCluster {
			if err := client.ExternalAuditStorageClient().DisableClusterExternalAuditStorage(ctx); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("cluster External Audit Storage configuration has been disabled\n")
		} else {
			if err := client.ExternalAuditStorageClient().DeleteDraftExternalAuditStorage(ctx); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("draft External Audit Storage configuration has been deleted\n")
		}
	case types.KindLock:
		name := rc.ref.Name
		if rc.ref.SubKind != "" {
			name = rc.ref.SubKind + "/" + name
		}
		if err = client.DeleteLock(ctx, name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("lock %q has been deleted\n", name)
	case types.KindDatabaseServer:
		servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "database server"
		servers = filterByNameOrDiscoveredName(servers, rc.ref.Name)
		name, err := getOneResourceNameToDelete(servers, rc.ref, resDesc)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, s := range servers {
			err := client.DeleteDatabaseServer(ctx, apidefaults.Namespace, s.GetHostID(), name)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		fmt.Printf("%s %q has been deleted\n", resDesc, name)
	case types.KindNetworkRestrictions:
		if err = resetNetworkRestrictions(ctx, client); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("network restrictions have been reset to defaults (allow all)\n")
	case types.KindApp:
		if err = client.DeleteApp(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("application %q has been deleted\n", rc.ref.Name)
	case types.KindDatabase:
		databases, err := client.GetDatabases(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "database"
		databases = filterByNameOrDiscoveredName(databases, rc.ref.Name)
		name, err := getOneResourceNameToDelete(databases, rc.ref, resDesc)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.DeleteDatabase(ctx, name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s %q has been deleted\n", resDesc, name)
	case types.KindKubernetesCluster:
		clusters, err := client.GetKubernetesClusters(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "Kubernetes cluster"
		clusters = filterByNameOrDiscoveredName(clusters, rc.ref.Name)
		name, err := getOneResourceNameToDelete(clusters, rc.ref, resDesc)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.DeleteKubernetesCluster(ctx, name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s %q has been deleted\n", resDesc, name)
	case types.KindCrownJewel:
		if err := client.CrownJewelsClient().DeleteCrownJewel(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown_jewel %q has been deleted\n", rc.ref.Name)
	case types.KindWindowsDesktopService:
		if err = client.DeleteWindowsDesktopService(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("windows desktop service %q has been deleted\n", rc.ref.Name)
	case types.KindDynamicWindowsDesktop:
		if err = client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("dynamic windows desktop %q has been deleted\n", rc.ref.Name)
	case types.KindWindowsDesktop:
		desktops, err := client.GetWindowsDesktops(ctx,
			types.WindowsDesktopFilter{Name: rc.ref.Name})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(desktops) == 0 {
			return trace.NotFound("no desktops with name %q were found", rc.ref.Name)
		}
		deleted := 0
		var errs []error
		for _, desktop := range desktops {
			if desktop.GetName() == rc.ref.Name {
				if err = client.DeleteWindowsDesktop(ctx, desktop.GetHostID(), rc.ref.Name); err != nil {
					errs = append(errs, err)
					continue
				}
				deleted++
			}
		}
		if deleted == 0 {
			errs = append(errs,
				trace.Errorf("failed to delete any desktops with the name %q, %d were found",
					rc.ref.Name, len(desktops)))
		}
		fmts := "%d windows desktops with name %q have been deleted"
		if err := trace.NewAggregate(errs...); err != nil {
			fmt.Printf(fmts+" with errors while deleting\n", deleted, rc.ref.Name)
			return err
		}
		fmt.Printf(fmts+"\n", deleted, rc.ref.Name)
	case types.KindCertAuthority:
		if rc.ref.SubKind == "" || rc.ref.Name == "" {
			return trace.BadParameter(
				"full %s path must be specified (e.g. '%s/%s/clustername')",
				types.KindCertAuthority, types.KindCertAuthority, types.HostCA,
			)
		}
		err := client.DeleteCertAuthority(ctx, types.CertAuthID{
			Type:       types.CertAuthType(rc.ref.SubKind),
			DomainName: rc.ref.Name,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s '%s/%s' has been deleted\n", types.KindCertAuthority, rc.ref.SubKind, rc.ref.Name)
	case types.KindKubeServer:
		servers, err := client.GetKubernetesServers(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "Kubernetes server"
		servers = filterByNameOrDiscoveredName(servers, rc.ref.Name)
		name, err := getOneResourceNameToDelete(servers, rc.ref, resDesc)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, s := range servers {
			err := client.DeleteKubernetesServer(ctx, s.GetHostID(), name)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		fmt.Printf("%s %q has been deleted\n", resDesc, name)
	case types.KindUIConfig:
		err := client.DeleteUIConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s has been deleted\n", types.KindUIConfig)
	case types.KindInstaller:
		err := client.DeleteInstaller(ctx, rc.ref.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if rc.ref.Name == installers.InstallerScriptName {
			fmt.Printf("%s has been reset to a default value\n", rc.ref.Name)
		} else {
			fmt.Printf("%s has been deleted\n", rc.ref.Name)
		}
	case types.KindLoginRule:
		loginRuleClient := client.LoginRuleClient()
		_, err := loginRuleClient.DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
			Name: rc.ref.Name,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
		fmt.Printf("login rule %q has been deleted\n", rc.ref.Name)
	case types.KindSAMLIdPServiceProvider:
		if err := client.DeleteSAMLIdPServiceProvider(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML IdP service provider %q has been deleted\n", rc.ref.Name)
	case types.KindDevice:
		remote := client.DevicesClient()
		device, err := findDeviceByIDOrTag(ctx, remote, rc.ref.Name)
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := remote.DeleteDevice(ctx, &devicepb.DeleteDeviceRequest{
			DeviceId: device[0].Id,
		}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Device %q removed\n", rc.ref.Name)

	case types.KindIntegration:
		if err := client.DeleteIntegration(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Integration %q removed\n", rc.ref.Name)

	case types.KindUserTask:
		if err := client.UserTasksServiceClient().DeleteUserTask(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user task %q has been deleted\n", rc.ref.Name)

	case types.KindDiscoveryConfig:
		remote := client.DiscoveryConfigClient()
		if err := remote.DeleteDiscoveryConfig(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("DiscoveryConfig %q removed\n", rc.ref.Name)

	case types.KindAppServer:
		appServers, err := client.GetApplicationServers(ctx, rc.namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		deleted := false
		for _, server := range appServers {
			if server.GetName() == rc.ref.Name {
				if err := client.DeleteApplicationServer(ctx, server.GetNamespace(), server.GetHostID(), server.GetName()); err != nil {
					return trace.Wrap(err)
				}
				deleted = true
			}
		}
		if !deleted {
			return trace.NotFound("application server %q not found", rc.ref.Name)
		}
		fmt.Printf("application server %q has been deleted\n", rc.ref.Name)
	case types.KindOktaImportRule:
		if err := client.OktaClient().DeleteOktaImportRule(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Okta import rule %q has been deleted\n", rc.ref.Name)
	case types.KindUserGroup:
		if err := client.DeleteUserGroup(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("User group %q has been deleted\n", rc.ref.Name)
	case types.KindProxy:
		if err := client.DeleteProxy(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Proxy %q has been deleted\n", rc.ref.Name)
	case types.KindAccessList:
		if err := client.AccessListClient().DeleteAccessList(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Access list %q has been deleted\n", rc.ref.Name)
	case types.KindAuditQuery:
		if err := client.SecReportsClient().DeleteSecurityAuditQuery(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Audit query %q has been deleted\n", rc.ref.Name)
	case types.KindSecurityReport:
		if err := client.SecReportsClient().DeleteSecurityReport(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Security report %q has been deleted\n", rc.ref.Name)
	case types.KindServerInfo:
		if err := client.DeleteServerInfo(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Server info %q has been deleted\n", rc.ref.Name)
	case types.KindBot:
		if _, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{BotName: rc.ref.Name}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Bot %q has been deleted\n", rc.ref.Name)
	case types.KindDatabaseObjectImportRule:
		if _, err := client.DatabaseObjectImportRuleClient().DeleteDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.DeleteDatabaseObjectImportRuleRequest{Name: rc.ref.Name}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Rule %q has been deleted\n", rc.ref.Name)
	case types.KindDatabaseObject:
		if err := client.DatabaseObjectsClient().DeleteDatabaseObject(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Object %q has been deleted\n", rc.ref.Name)
	case types.KindAccessMonitoringRule:
		if err := client.AccessMonitoringRuleClient().DeleteAccessMonitoringRule(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Access monitoring rule %q has been deleted\n", rc.ref.Name)
	case types.KindSPIFFEFederation:
		if _, err := client.SPIFFEFederationServiceClient().DeleteSPIFFEFederation(
			ctx, &machineidv1pb.DeleteSPIFFEFederationRequest{
				Name: rc.ref.Name,
			},
		); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SPIFFE federation %q has been deleted\n", rc.ref.Name)
	case types.KindWorkloadIdentity:
		if _, err := client.WorkloadIdentityResourceServiceClient().DeleteWorkloadIdentity(
			ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: rc.ref.Name,
			}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Workload identity %q has been deleted\n", rc.ref.Name)
	case types.KindWorkloadIdentityX509Revocation:
		if _, err := client.WorkloadIdentityRevocationServiceClient().DeleteWorkloadIdentityX509Revocation(
			ctx, &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: rc.ref.Name,
			}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Workload identity X509 revocation %q has been deleted\n", rc.ref.Name)
	case types.KindWorkloadIdentityX509IssuerOverride:
		c := client.WorkloadIdentityX509OverridesClient()
		if _, err := c.DeleteX509IssuerOverride(
			ctx,
			&workloadidentityv1pb.DeleteX509IssuerOverrideRequest{
				Name: rc.ref.Name,
			},
		); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(
			rc.stdout,
			types.KindWorkloadIdentityX509IssuerOverride+" %q has been deleted\n",
			rc.ref.Name,
		)
	case types.KindSigstorePolicy:
		c := client.SigstorePolicyResourceServiceClient()
		if _, err := c.DeleteSigstorePolicy(
			ctx,
			&workloadidentityv1pb.DeleteSigstorePolicyRequest{
				Name: rc.ref.Name,
			},
		); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(
			rc.stdout,
			types.KindSigstorePolicy+" %q has been deleted\n",
			rc.ref.Name,
		)
	case types.KindStaticHostUser:
		if err := client.StaticHostUserClient().DeleteStaticHostUser(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("static host user %q has been deleted\n", rc.ref.Name)
	case types.KindGitServer:
		if err := client.GitServerClient().DeleteGitServer(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("git_server %q has been deleted\n", rc.ref.Name)
	case types.KindAutoUpdateConfig:
		if err := client.DeleteAutoUpdateConfig(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("AutoUpdateConfig has been deleted\n")
	case types.KindAutoUpdateVersion:
		if err := client.DeleteAutoUpdateVersion(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("AutoUpdateVersion has been deleted\n")
	case types.KindAutoUpdateAgentRollout:
		if err := client.DeleteAutoUpdateAgentRollout(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("AutoUpdateAgentRollout has been deleted\n")
	case types.KindHealthCheckConfig:
		return trace.Wrap(rc.deleteHealthCheckConfig(ctx, client))
	default:
		return trace.BadParameter("deleting resources of type %q is not supported", rc.ref.Kind)
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

	if getter, found := rc.GetHandlers[ResourceKind(rc.ref.Kind)]; found {
		return getter(ctx, client)
	}
	return nil, trace.BadParameter("getting %q is not supported", rc.ref.String())
}

func (rc *ResourceCommand) getSAMLConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetSAMLConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSAMLCollection(connectors), nil
	}
	connector, err := client.GetSAMLConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSAMLCollection([]types.SAMLConnector{connector}), nil
}

func (rc *ResourceCommand) getOIDCConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetOIDCConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewOIDCCollection(connectors), nil
	}
	connector, err := client.GetOIDCConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewOIDCCollection([]types.OIDCConnector{connector}), nil
}

func (rc *ResourceCommand) getGithubConnectors(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name == "" {
		connectors, err := client.GetGithubConnectors(ctx, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewGithubCollection(connectors), nil
	}
	connector, err := client.GetGithubConnector(ctx, rc.ref.Name, rc.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewGithubCollection([]types.GithubConnector{connector}), nil
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

func findDeviceByIDOrTag(ctx context.Context, remote devicepb.DeviceTrustServiceClient, idOrTag string) ([]*devicepb.Device, error) {
	resp, err := remote.FindDevices(ctx, &devicepb.FindDevicesRequest{
		IdOrTag: idOrTag,
	})
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case len(resp.Devices) == 0:
		return nil, trace.NotFound("device %q not found", idOrTag)
	case len(resp.Devices) == 1:
		return resp.Devices, nil
	}

	// Do we have an ID match?
	for _, dev := range resp.Devices {
		if dev.Id == idOrTag {
			return []*devicepb.Device{dev}, nil
		}
	}

	return nil, trace.BadParameter("found multiple devices for asset tag %q, please retry using the device ID instead", idOrTag)
}

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
