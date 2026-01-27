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

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/itertools/stream"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

// ResourceCreateHandler is the generic implementation of a resource creation handler
type ResourceCreateHandler func(context.Context, *authclient.Client, services.UnknownResource) error

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
	deleteCmd    *kingpin.CmdClause
	getCmd       *kingpin.CmdClause
	createCmd    *kingpin.CmdClause
	updateCmd    *kingpin.CmdClause
	listKindsCmd *kingpin.CmdClause

	verbose bool

	CreateHandlers map[string]ResourceCreateHandler
	UpdateHandlers map[string]ResourceCreateHandler

	// Stdout allows to switch standard output source for resource command. Used in tests.
	Stdout io.Writer
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
	rc.CreateHandlers = map[string]ResourceCreateHandler{
		types.KindTrustedCluster:              rc.createTrustedCluster,
		types.KindExternalAuditStorage:        rc.createExternalAuditStorage,
		types.KindNetworkRestrictions:         rc.createNetworkRestrictions,
		types.KindLoginRule:                   rc.createLoginRule,
		types.KindDevice:                      rc.createDevice,
		types.KindOktaImportRule:              rc.createOktaImportRule,
		types.KindIntegration:                 rc.createIntegration,
		types.KindSecurityReport:              rc.createSecurityReport,
		types.KindCrownJewel:                  rc.createCrownJewel,
		types.KindVnetConfig:                  rc.createVnetConfig,
		types.KindPlugin:                      rc.createPlugin,
		types.KindHealthCheckConfig:           rc.createHealthCheckConfig,
		scopedaccess.KindScopedRole:           rc.createScopedRole,
		scopedaccess.KindScopedRoleAssignment: rc.createScopedRoleAssignment,
		types.KindInferencePolicy:             rc.createInferencePolicy,
	}
	rc.UpdateHandlers = map[string]ResourceCreateHandler{
		types.KindCrownJewel:                  rc.updateCrownJewel,
		types.KindVnetConfig:                  rc.updateVnetConfig,
		types.KindPlugin:                      rc.updatePlugin,
		types.KindHealthCheckConfig:           rc.updateHealthCheckConfig,
		scopedaccess.KindScopedRole:           rc.updateScopedRole,
		scopedaccess.KindScopedRoleAssignment: rc.updateScopedRoleAssignment,
		types.KindInferencePolicy:             rc.updateInferencePolicy,
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

	rc.deleteCmd = app.Command("rm", "Delete a resource.").Alias("del").Alias("delete")
	rc.deleteCmd.Arg("resource type/resource name", `Resource to delete
	<resource type>  Type of a resource [for example: connector,user,cluster,token]
	<resource name>  Resource name to delete

	Examples:
	$ tctl rm role/devs
	$ tctl rm cluster/main`).SetValue(&rc.ref)

	rc.getCmd = app.Command("get", "Print a YAML declaration of various Teleport resources.")
	rc.getCmd.Arg("resources", "Resource spec: 'type/[name][,...]' or 'all'").Required().SetValue(&rc.refs)
	rc.getCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&rc.format)
	rc.getCmd.Flag("namespace", "Namespace of the resources").Hidden().Default(apidefaults.Namespace).StringVar(&rc.namespace)
	rc.getCmd.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&rc.withSecrets)
	rc.getCmd.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&rc.verbose)

	rc.getCmd.Alias(getHelp)

	rc.listKindsCmd = app.Command("list-kinds", "Lists all resource kinds supported by this tctl version.").Alias("api-resources")
	rc.listKindsCmd.Flag("wide", "Do not truncate the Description column, even if it exceeds terminal width").BoolVar(&rc.verbose)

	if rc.Stdout == nil {
		rc.Stdout = os.Stdout
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
	case rc.listKindsCmd.FullCommand():
		// early return before we try to build a client
		// we don't need a valid client to run this command
		return true, trace.Wrap(rc.listKinds())
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
	mfaKinds := []string{types.KindCertAuthority}
	for kind, handler := range resources.Handlers() {
		if handler.MFARequired() {
			mfaKinds = append(mfaKinds, kind)
		}
	}
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
		return collection.WriteText(rc.Stdout, rc.verbose)
	case teleport.YAML:
		return writeYAML(collection, rc.Stdout)
	case teleport.JSON:
		return writeJSON(collection, rc.Stdout)
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
	if err := utils.WriteYAML(rc.Stdout, resources); err != nil {
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

		// Try looking for a resource handler
		if resourceHandler, found := resources.Handlers()[raw.Kind]; found {
			// only return in case of error, to create multiple resources
			// in case if yaml spec is a list
			opts := resources.CreateOpts{
				Force:   rc.force,
				Confirm: rc.confirm,
			}
			if err := resourceHandler.Create(ctx, client, raw, opts); err != nil {
				if trace.IsAlreadyExists(err) {
					return trace.Wrap(err, "use -f or --force flag to overwrite")
				}
				if trace.IsNotImplemented(err) {
					return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
				}
				return trace.Wrap(err)
			}
			// continue to next resource
			continue
		}
		// Else fallback to the legacy logic

		// locate the creator function for a given resource kind:
		creator, found := rc.CreateHandlers[raw.Kind]
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

// createTrustedCluster implements `tctl create cluster.yaml` command
func (rc *ResourceCommand) createTrustedCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	tc, err := services.UnmarshalTrustedCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// check if such cluster already exists:
	name := tc.GetName()
	_, err = client.GetTrustedCluster(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !rc.force && exists {
		return trace.AlreadyExists("trusted cluster %q already exists", name)
	}

	//nolint:staticcheck // SA1019. UpsertTrustedCluster is deprecated but will
	// continue being supported for tctl clients.
	// TODO(bernardjkim) consider using UpsertTrustedClusterV2 in VX.0.0
	out, err := client.UpsertTrustedCluster(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if out.GetName() != tc.GetName() {
		fmt.Printf("WARNING: trusted cluster resource %q has been renamed to match root cluster name %q. this will become an error in future teleport versions, please update your configuration to use the correct name.\n", name, out.GetName())
	}
	fmt.Printf("trusted cluster %q has been %v\n", out.GetName(), UpsertVerb(exists, rc.force))
	return nil
}

// createExternalAuditStorage implements `tctl create external_audit_storage` command.
func (rc *ResourceCommand) createExternalAuditStorage(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	draft, err := services.UnmarshalExternalAuditStorage(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	externalAuditClient := client.ExternalAuditStorageClient()
	if rc.force {
		if _, err := externalAuditClient.UpsertDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been updated\n")
	} else {
		if _, err := externalAuditClient.CreateDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been created\n")
	}
	return nil
}

// createNetworkRestrictions implements `tctl create net_restrict.yaml` command.
func (rc *ResourceCommand) createNetworkRestrictions(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newNetRestricts, err := services.UnmarshalNetworkRestrictions(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.SetNetworkRestrictions(ctx, newNetRestricts); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("network restrictions have been updated\n")
	return nil
}

func (rc *ResourceCommand) createCrownJewel(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	crownJewel, err := services.UnmarshalCrownJewel(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.CrownJewelsClient()
	if rc.force {
		if _, err := c.UpsertCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been updated\n", crownJewel.GetMetadata().GetName())
	} else {
		if _, err := c.CreateCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been created\n", crownJewel.GetMetadata().GetName())
	}

	return nil
}

func (rc *ResourceCommand) createScopedRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	if rc.IsForced() {
		return trace.BadParameter("scoped role creation does not support --force")
	}

	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRole](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.ScopedAccessServiceClient().CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: r,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(
		rc.Stdout,
		scopedaccess.KindScopedRole+" %q has been created\n",
		r.GetMetadata().GetName(),
	)

	return nil
}

func (rc *ResourceCommand) updateScopedRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRole](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ScopedAccessServiceClient().UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: r,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(
		rc.Stdout,
		scopedaccess.KindScopedRole+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)

	return nil
}

func (rc *ResourceCommand) createScopedRoleAssignment(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	if rc.IsForced() {
		return trace.BadParameter("scoped role assignment creation does not support --force")
	}

	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRoleAssignment](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	rsp, err := client.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: r,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(
		rc.Stdout,
		scopedaccess.KindScopedRoleAssignment+" %q has been created\n",
		rsp.GetAssignment().GetMetadata().GetName(), // must extract from rsp since assignment names are generated server-side
	)

	return nil
}

func (rc *ResourceCommand) updateScopedRoleAssignment(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	return trace.NotImplemented("scoped_role_assignment resources do not support updates")
}

func (rc *ResourceCommand) updateCrownJewel(ctx context.Context, client *authclient.Client, resource services.UnknownResource) error {
	in, err := services.UnmarshalCrownJewel(resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.CrownJewelsClient().UpdateCrownJewel(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown jewel %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) createLoginRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	rule, err := loginrule.UnmarshalLoginRule(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	loginRuleClient := client.LoginRuleClient()
	if rc.IsForced() {
		_, err := loginRuleClient.UpsertLoginRule(ctx, &loginrulepb.UpsertLoginRuleRequest{
			LoginRule: rule,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
	} else {
		_, err = loginRuleClient.CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
			LoginRule: rule,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
	}
	verb := UpsertVerb(false /* we don't know if it existed before */, rc.IsForced() /* force update */)
	fmt.Printf("login_rule %q has been %s\n", rule.GetMetadata().GetName(), verb)
	return nil
}

func (rc *ResourceCommand) createDevice(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	res, err := services.UnmarshalDevice(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	dev, err := types.DeviceFromResource(res)
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.IsForced() {
		_, err = client.DevicesClient().UpsertDevice(ctx, &devicepb.UpsertDeviceRequest{
			Device:           dev,
			CreateAsResource: true,
		})
		// err checked below
	} else {
		_, err = client.DevicesClient().CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device:           dev,
			CreateAsResource: true,
		})
		// err checked below
	}
	if err != nil {
		return trail.FromGRPC(err)
	}

	verb := "created"
	if rc.IsForced() {
		verb = "updated"
	}

	fmt.Printf("Device %v/%v %v\n",
		dev.AssetTag,
		devicetrust.FriendlyOSType(dev.OsType),
		verb,
	)
	return nil
}

func (rc *ResourceCommand) createOktaImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	importRule, err := services.UnmarshalOktaImportRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	exists := false
	if _, err = client.OktaClient().CreateOktaImportRule(ctx, importRule); err != nil {
		if trace.IsAlreadyExists(err) {
			exists = true
			_, err = client.OktaClient().UpdateOktaImportRule(ctx, importRule)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("Okta import rule %q has been %s\n", importRule.GetName(), UpsertVerb(exists, rc.IsForced()))
	return nil
}

func (rc *ResourceCommand) createIntegration(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	integration, err := services.UnmarshalIntegration(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	existingIntegration, err := client.GetIntegration(ctx, integration.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !rc.force {
			return trace.AlreadyExists("Integration %q already exists", integration.GetName())
		}

		if err := existingIntegration.CanChangeStateTo(integration); err != nil {
			return trace.Wrap(err)
		}

		switch integration.GetSubKind() {
		case types.IntegrationSubKindAWSOIDC:
			existingIntegration.SetAWSOIDCIntegrationSpec(integration.GetAWSOIDCIntegrationSpec())
		case types.IntegrationSubKindGitHub:
			existingIntegration.SetGitHubIntegrationSpec(integration.GetGitHubIntegrationSpec())
		case types.IntegrationSubKindAWSRolesAnywhere:
			existingIntegration.SetAWSRolesAnywhereIntegrationSpec(integration.GetAWSRolesAnywhereIntegrationSpec())
		case types.IntegrationSubKindAzureOIDC:
			existingIntegration.SetAzureOIDCIntegrationSpec(integration.GetAzureOIDCIntegrationSpec())
		default:
			return trace.BadParameter("subkind %q is not supported", integration.GetSubKind())
		}

		if _, err := client.UpdateIntegration(ctx, existingIntegration); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Integration %q has been updated\n", integration.GetName())
		return nil
	}

	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return trace.BadParameter("unexpected Integration type %T", integration)
	}

	if _, err := client.CreateIntegration(ctx, igV1); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Integration %q has been created\n", integration.GetName())

	return nil
}

// Delete deletes resource by name
func (rc *ResourceCommand) Delete(ctx context.Context, client *authclient.Client) (err error) {
	// Connectors are a special case. As it's the only meta-resource we have,
	// it's easier to special-case it here instead of adding a case in the
	// generic [resources.Handler].
	if rc.ref.Kind == types.KindConnectors {
		return trace.BadParameter(
			"Deleting connector resources requires using an explicit connector type. Please try again with the appropriate type: %s",
			[]string{
				types.KindGithubConnector + "/" + rc.ref.Name,
				types.KindOIDCConnector + "/" + rc.ref.Name,
				types.KindSAMLConnector + "/" + rc.ref.Name,
			},
		)
	}

	// Try looking for a resource handler
	if resourceHandler, found := resources.Handlers()[rc.ref.Kind]; found {
		if err := resourceHandler.Delete(ctx, client, rc.ref); err != nil {
			if trace.IsNotImplemented(err) {
				return trace.BadParameter("deleting resources of type %q is not supported", rc.ref.Kind)
			}
			return trace.Wrap(err, "error deleting resource %q of type %q", rc.ref.Name, rc.ref.Kind)
		}
		return nil
	}

	// Else fallback to the legacy logic
	singletonResources := []string{
		types.KindSessionRecordingConfig,
		types.KindInstaller,
		types.KindUIConfig,
		types.KindNetworkRestrictions,
	}
	if !slices.Contains(singletonResources, rc.ref.Kind) && (rc.ref.Kind == "" || rc.ref.Name == "") {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}

	switch rc.ref.Kind {
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
	case types.KindDatabaseServer:
		servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "database server"
		servers = resources.FilterByNameOrDiscoveredName(servers, rc.ref.Name)
		name, err := resources.GetOneResourceNameToDelete(servers, rc.ref, resDesc)
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
	case types.KindCrownJewel:
		if err := client.CrownJewelsClient().DeleteCrownJewel(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown_jewel %q has been deleted\n", rc.ref.Name)
	case types.KindLoginRule:
		loginRuleClient := client.LoginRuleClient()
		_, err := loginRuleClient.DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
			Name: rc.ref.Name,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
		fmt.Printf("login rule %q has been deleted\n", rc.ref.Name)
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
	case types.KindOktaAssignment:
		if err := client.OktaClient().DeleteOktaAssignment(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Okta assignment %q has been deleted\n", rc.ref.Name)
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
	case types.KindSecurityReport:
		if err := client.SecReportsClient().DeleteSecurityReport(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Security report %q has been deleted\n", rc.ref.Name)
	case scopedaccess.KindScopedRole:
		if _, err := client.ScopedAccessServiceClient().DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
			Name: rc.ref.Name,
		}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(
			rc.Stdout,
			scopedaccess.KindScopedRole+" %q has been deleted\n",
			rc.ref.Name,
		)
	case scopedaccess.KindScopedRoleAssignment:
		if _, err := client.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
			Name: rc.ref.Name,
		}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(
			rc.Stdout,
			scopedaccess.KindScopedRoleAssignment+" %q has been deleted\n",
			rc.ref.Name,
		)
	case types.KindHealthCheckConfig:
		return trace.Wrap(rc.deleteHealthCheckConfig(ctx, client))
	case types.KindInferencePolicy:
		return trace.Wrap(rc.deleteInferencePolicy(ctx, client))
	default:
		return trace.BadParameter("deleting resources of type %q is not supported", rc.ref.Kind)
	}
	return nil
}

func resetNetworkRestrictions(ctx context.Context, client *authclient.Client) error {
	return trace.Wrap(client.DeleteNetworkRestrictions(ctx))
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
func (rc *ResourceCommand) getCollection(ctx context.Context, client *authclient.Client) (resources.Collection, error) {
	if rc.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}

	// Looking if the resource has been converted to the handler format.
	if handler, found := resources.Handlers()[rc.ref.Kind]; found {
		coll, err := handler.Get(ctx, client, rc.ref, resources.GetOpts{WithSecrets: rc.withSecrets})
		if err != nil {
			if trace.IsNotImplemented(err) {
				return nil, trace.BadParameter("getting %q is not supported", rc.ref.String())
			}
			return nil, trace.Wrap(err, "getting resource %q of type %q", rc.ref.Name, rc.ref.Kind)
		}
		return coll, nil
	}
	// The resource hasn't been migrated yet, falling back to the old logic.

	switch rc.ref.Kind {
	case types.KindReverseTunnel:
		if rc.ref.Name != "" {
			return nil, trace.BadParameter("reverse tunnel cannot be searched by name")
		}

		tunnels, err := stream.Collect(clientutils.Resources(ctx, client.ListReverseTunnels))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &reverseTunnelCollection{tunnels: tunnels}, nil
	case types.KindTrustedCluster:
		if rc.ref.Name == "" {
			// TODO(okraport): DELETE IN v21.0.0, replace with regular Collect
			trustedClusters, err := clientutils.CollectWithFallback(
				ctx,
				client.ListTrustedClusters,
				client.GetTrustedClusters,
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
		}
		trustedCluster, err := client.GetTrustedCluster(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []types.TrustedCluster{trustedCluster}}, nil
	case types.KindRemoteCluster:
		if rc.ref.Name == "" {
			remoteClusters, err := client.GetRemoteClusters(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &remoteClusterCollection{remoteClusters: remoteClusters}, nil
		}
		remoteCluster, err := client.GetRemoteCluster(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &remoteClusterCollection{remoteClusters: []types.RemoteCluster{remoteCluster}}, nil
	case types.KindSemaphore:
		filter := types.SemaphoreFilter{
			SemaphoreKind: rc.ref.SubKind,
			SemaphoreName: rc.ref.Name,
		}
		sems, err := clientutils.CollectWithFallback(ctx,
			func(ctx context.Context, pageSize int, pageToken string) ([]types.Semaphore, string, error) {
				return client.ListSemaphores(ctx, pageSize, pageToken, &filter)
			},
			func(ctx context.Context) ([]types.Semaphore, error) {
				return client.GetSemaphores(ctx, filter)
			},
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &semaphoreCollection{sems: sems}, nil
	case types.KindSessionRecordingConfig:
	case types.KindDatabaseServer:
		servers, err := client.GetDatabaseServers(ctx, rc.namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if rc.ref.Name == "" {
			return &databaseServerCollection{servers: servers}, nil
		}

		servers = resources.FilterByNameOrDiscoveredName(servers, rc.ref.Name)
		if len(servers) == 0 {
			return nil, trace.NotFound("database server %q not found", rc.ref.Name)
		}
		return &databaseServerCollection{servers: servers}, nil
	case types.KindNetworkRestrictions:
		nr, err := client.GetNetworkRestrictions(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &netRestrictionsCollection{nr}, nil
	case types.KindCrownJewel:
		jewels, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]*crownjewelv1.CrownJewel, string, error) {
			return client.CrownJewelsClient().ListCrownJewels(ctx, int64(limit), startKey)
		}))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &crownJewelCollection{items: jewels}, nil
	case types.KindToken:
	case types.KindDatabaseService:
		resourceName := rc.ref.Name
		listReq := proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
		}
		if resourceName != "" {
			listReq.PredicateExpression = fmt.Sprintf(`name == "%s"`, resourceName)
		}

		getResp, err := apiclient.GetResourcesWithFilters(ctx, client, listReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		databaseServices, err := types.ResourcesWithLabels(getResp).AsDatabaseServices()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(databaseServices) == 0 && resourceName != "" {
			return nil, trace.NotFound("Database Service %q not found", resourceName)
		}

		return &databaseServiceCollection{databaseServices: databaseServices}, nil
	case types.KindLoginRule:
		loginRuleClient := client.LoginRuleClient()
		if rc.ref.Name == "" {
			rules, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*loginrulepb.LoginRule, string, error) {
				resp, err := loginRuleClient.ListLoginRules(ctx, &loginrulepb.ListLoginRulesRequest{
					PageSize:  int32(limit),
					PageToken: token,
				})
				return resp.GetLoginRules(), resp.GetNextPageToken(), trace.Wrap(err)
			}))
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &loginRuleCollection{rules}, trace.Wrap(err)
		}
		rule, err := loginRuleClient.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
			Name: rc.ref.Name,
		})
		return &loginRuleCollection{[]*loginrulepb.LoginRule{rule}}, trail.FromGRPC(err)
	case types.KindDevice:
		remote := client.DevicesClient()
		if rc.ref.Name != "" {
			resp, err := remote.FindDevices(ctx, &devicepb.FindDevicesRequest{
				IdOrTag: rc.ref.Name,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &deviceCollection{resp.Devices}, nil
		}

		req := &devicepb.ListDevicesRequest{
			View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		}
		var devs []*devicepb.Device
		for {
			resp, err := remote.ListDevices(ctx, req)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			devs = append(devs, resp.Devices...)

			if resp.NextPageToken == "" {
				break
			}
			req.PageToken = resp.NextPageToken
		}

		sort.Slice(devs, func(i, j int) bool {
			d1 := devs[i]
			d2 := devs[j]

			if d1.AssetTag == d2.AssetTag {
				return d1.OsType < d2.OsType
			}

			return d1.AssetTag < d2.AssetTag
		})

		return &deviceCollection{devices: devs}, nil
	case types.KindOktaImportRule:
		if rc.ref.Name != "" {
			importRule, err := client.OktaClient().GetOktaImportRule(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &oktaImportRuleCollection{importRules: []types.OktaImportRule{importRule}}, nil
		}

		resources, err := stream.Collect(clientutils.Resources(ctx, client.OktaClient().ListOktaImportRules))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &oktaImportRuleCollection{importRules: resources}, nil
	case types.KindOktaAssignment:
		if rc.ref.Name != "" {
			assignment, err := client.OktaClient().GetOktaAssignment(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &oktaAssignmentCollection{assignments: []types.OktaAssignment{assignment}}, nil
		}

		resources, err := stream.Collect(clientutils.Resources(ctx, client.OktaClient().ListOktaAssignments))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &oktaAssignmentCollection{assignments: resources}, nil
	case types.KindUserGroup:
		if rc.ref.Name != "" {
			userGroup, err := client.GetUserGroup(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &userGroupCollection{userGroups: []types.UserGroup{userGroup}}, nil
		}

		resources, err := stream.Collect(clientutils.Resources(ctx, client.ListUserGroups))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &userGroupCollection{userGroups: resources}, nil
	case types.KindExternalAuditStorage:
		out := []*externalauditstorage.ExternalAuditStorage{}
		name := rc.ref.Name
		switch name {
		case "":
			cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
			if err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			} else {
				out = append(out, cluster)
			}
			draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
			if err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
			} else {
				out = append(out, draft)
			}
			return &externalAuditStorageCollection{externalAuditStorages: out}, nil
		case types.MetaNameExternalAuditStorageCluster:
			cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &externalAuditStorageCollection{externalAuditStorages: []*externalauditstorage.ExternalAuditStorage{cluster}}, nil
		case types.MetaNameExternalAuditStorageDraft:
			draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &externalAuditStorageCollection{externalAuditStorages: []*externalauditstorage.ExternalAuditStorage{draft}}, nil
		default:
			return nil, trace.BadParameter("unsupported resource name for external_audit_storage, valid for get are: '', %q, %q", types.MetaNameExternalAuditStorageDraft, types.MetaNameExternalAuditStorageCluster)
		}
	case types.KindIntegration:
		if rc.ref.Name != "" {
			ig, err := client.GetIntegration(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &integrationCollection{integrations: []types.Integration{ig}}, nil
		}

		resources, err := stream.Collect(clientutils.Resources(ctx, client.ListIntegrations))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &integrationCollection{integrations: resources}, nil
	case types.KindSecurityReport:
		if rc.ref.Name != "" {

			resource, err := client.SecReportsClient().GetSecurityReport(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &securityReportCollection{items: []*secreports.Report{resource}}, nil
		}
		resources, err := client.SecReportsClient().GetSecurityReports(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &securityReportCollection{items: resources}, nil
	case types.KindVnetConfig:
		vnetConfig, err := client.VnetConfigServiceClient().GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &vnetConfigCollection{vnetConfig: vnetConfig}, nil
	case types.KindPlugin:
		if rc.ref.Name != "" {
			plugin, err := client.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: rc.ref.Name})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &pluginCollection{plugins: []types.Plugin{plugin}}, nil
		}
		var plugins []types.Plugin
		startKey := ""
		for {
			resp, err := client.PluginsClient().ListPlugins(ctx, &pluginsv1.ListPluginsRequest{
				PageSize:    100,
				StartKey:    startKey,
				WithSecrets: rc.withSecrets,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			for _, v := range resp.Plugins {
				plugins = append(plugins, v)
			}
			if resp.NextKey == "" {
				break
			}
			startKey = resp.NextKey
		}
		return &pluginCollection{plugins: plugins}, nil

	case types.KindHealthCheckConfig:
		if rc.ref.Name != "" {
			cfg, err := client.GetHealthCheckConfig(ctx, rc.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &healthCheckConfigCollection{
				items: []*healthcheckconfigv1.HealthCheckConfig{cfg},
			}, nil
		}

		items, err := stream.Collect(clientutils.Resources(ctx, client.ListHealthCheckConfigs))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &healthCheckConfigCollection{items: items}, nil
	case scopedaccess.KindScopedRole:
		if rc.ref.Name != "" {
			rsp, err := client.ScopedAccessServiceClient().GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
				Name: rc.ref.Name,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &scopedRoleCollection{items: []*scopedaccessv1.ScopedRole{rsp.Role}}, nil
		}

		items, err := stream.Collect(scopedutils.RangeScopedRoles(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRolesRequest{}))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &scopedRoleCollection{items: items}, nil
	case scopedaccess.KindScopedRoleAssignment:
		if rc.ref.Name != "" {
			rsp, err := client.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
				Name: rc.ref.Name,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &scopedRoleAssignmentCollection{items: []*scopedaccessv1.ScopedRoleAssignment{rsp.Assignment}}, nil
		}

		items, err := stream.Collect(scopedutils.RangeScopedRoleAssignments(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRoleAssignmentsRequest{}))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &scopedRoleAssignmentCollection{items: items}, nil
	case types.KindInferencePolicy:
		policies, err := rc.getInferencePolicies(ctx, client)
		return policies, trace.Wrap(err)
	}
	return nil, trace.BadParameter("getting %q is not supported", rc.ref.String())
}

// UpsertVerb generates the correct string form of a verb based on the action taken
func UpsertVerb(exists bool, force bool) string {
	if !force && exists {
		return "updated"
	}
	return "created"
}

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

func (rc *ResourceCommand) createSecurityReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalSecurityReport(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := in.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err = client.SecReportsClient().UpsertSecurityReport(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) createVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.IsForced() {
		_, err = client.VnetConfigServiceClient().UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
	} else {
		_, err = client.VnetConfigServiceClient().CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("vnet_config has been created")
	return nil
}

func (rc *ResourceCommand) updateVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.VnetConfigServiceClient().UpdateVnetConfig(ctx, &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("vnet_config has been updated")
	return nil
}

func (rc *ResourceCommand) updatePlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	item := pluginResourceWrapper{PluginV1: types.PluginV1{}}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{Plugin: &item.PluginV1}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) createPlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	item := pluginResourceWrapper{
		PluginV1: types.PluginV1{},
	}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if !rc.IsForced() {
		// Plugin needs to be installed before it can be updated.
		return trace.BadParameter("Only plugin update operation is supported. Please use 'tctl plugins install' instead\n")
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{Plugin: &item.PluginV1}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("plugin %q has been updated\n", item.GetName())
	return nil
}

// UpdateFields updates select resource fields: expiry and labels
func (rc *ResourceCommand) listKinds() error {
	// We must compute rows before, and cannot add them as we go
	// because this breaks the "truncated columns behavior"
	var rows [][]string
	for kind, handler := range resources.Handlers() {
		rows = append(rows, []string{
			kind,
			strings.Join(handler.SupportedCommands(), ","),
			yesOrEmpty(handler.Singleton()),
			yesOrEmpty(handler.MFARequired()),
			handler.Description(),
		})
	}

	var t asciitable.Table
	headers := []string{"Kind", "Supported Commands", "Singleton", "MFA", "Description"}
	if rc.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}
	t.SortRowsBy([]int{0}, true)
	return trace.Wrap(t.WriteTo(rc.Stdout))
}

func yesOrEmpty(b bool) string {
	if b {
		return "yes"
	}
	return ""
}
