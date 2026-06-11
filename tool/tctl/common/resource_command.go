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
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

// ResourceCreateHandler is the generic implementation of a resource creation handler
type ResourceCreateHandler func(context.Context, *authclient.Client, services.UnknownResource) error

// ResourceCommand implements `tctl get/create/list` commands for manipulating
// Teleport resources
type ResourceCommand struct {
	config      *servicecfg.Config
	ref         string
	id          string
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
		types.KindTrustedCluster:      rc.createTrustedCluster,
		types.KindNetworkRestrictions: rc.createNetworkRestrictions,
		types.KindDevice:              rc.createDevice,
		types.KindOktaImportRule:      rc.createOktaImportRule,
		types.KindIntegration:         rc.createIntegration,
		types.KindSecurityReport:      rc.createSecurityReport,
		types.KindCrownJewel:          rc.createCrownJewel,
		types.KindPlugin:              rc.createPlugin,
		types.KindHealthCheckConfig:   rc.createHealthCheckConfig,
	}
	rc.UpdateHandlers = map[string]ResourceCreateHandler{
		types.KindCrownJewel:        rc.updateCrownJewel,
		types.KindPlugin:            rc.updatePlugin,
		types.KindHealthCheckConfig: rc.updateHealthCheckConfig,
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
	$ tctl update rc/remote`).StringVar(&rc.ref)
	rc.updateCmd.Arg("id", `Resource identifier: scope-qualified name (e.g. "/staging/west::myrole") or bare name for unscoped kinds`).StringVar(&rc.id)
	rc.updateCmd.Flag("set-labels", "Set labels").StringVar(&rc.labels)
	rc.updateCmd.Flag("set-ttl", "Set TTL").StringVar(&rc.ttl)

	rc.deleteCmd = app.Command("rm", "Delete a resource.").Alias("del").Alias("delete")
	rc.deleteCmd.Arg("resource type/resource name", `Resource to delete
	<resource type>  Type of a resource [for example: connector,user,cluster,token]
	<resource name>  Resource name to delete

	Examples:
	$ tctl rm role/devs
	$ tctl rm cluster/main`).StringVar(&rc.ref)
	rc.deleteCmd.Arg("id", `Resource identifier: scope-qualified name (e.g. "/staging/west::myrole") or bare name for unscoped kinds`).StringVar(&rc.id)

	rc.getCmd = app.Command("get", "Print a YAML declaration of various Teleport resources.")
	rc.getCmd.Arg("resources", "Resource spec: 'type/[name][,...]' or 'all'").Required().StringVar(&rc.ref)
	rc.getCmd.Arg("id", `Resource identifier: scope-qualified name (e.g. "/staging/west::myrole") or bare name for unscoped kinds`).StringVar(&rc.id)
	rc.getCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&rc.format)
	rc.getCmd.Flag("namespace", "Namespace of the resources").Hidden().Default(apidefaults.Namespace).StringVar(&rc.namespace)
	rc.getCmd.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&rc.withSecrets)
	rc.getCmd.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&rc.verbose)

	rc.getCmd.Alias(getHelp)

	rc.listKindsCmd = app.Command("list-kinds", "Lists all resource kinds supported by this tctl version.").Alias("api-resources")
	rc.listKindsCmd.Flag("wide", "Do not truncate the Description column, even if it exceeds terminal width").BoolVar(&rc.verbose)
	rc.listKindsCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&rc.format, teleport.Text, teleport.JSON, teleport.YAML)

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
	for kind, handler := range resources.ScopedHandlers() {
		if handler.MFARequired() {
			mfaKinds = append(mfaKinds, kind)
		}
	}

	// performMFAIfNeeded runs the MFA ceremony when required is true and MFA
	// has not already been provided. It updates ctx in-place on success.
	performMFAIfNeeded := func(required bool) error {
		if !required {
			return nil
		}
		if _, err := mfa.MFAResponseFromContext(ctx); err == nil {
			return nil // already provided
		}
		mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
		if err == nil {
			ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
		} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
			return trace.Wrap(err)
		}
		return nil
	}

	// writeCollection writes a collection to rc.Stdout in the requested format.
	// Note that only YAML is officially supported; text and JSON are experimental.
	writeCollection := func(coll resources.Collection) error {
		switch rc.format {
		case teleport.Text:
			return coll.WriteText(rc.Stdout, rc.verbose)
		case teleport.YAML:
			return writeYAML(coll, rc.Stdout)
		case teleport.JSON:
			return writeJSON(coll, rc.Stdout)
		}
		return trace.BadParameter("unsupported format")
	}

	// When a second positional arg (id) is present, short-circuit to the
	// single-resource/scoped-resource path (note: this path relies on ScopedRef,
	// but the underlying logic supports lookup of scoped and unscoped resources).
	if rc.id != "" {
		sr, err := ParseScopedRef(rc.ref, rc.id)
		if err != nil {
			return trace.Wrap(err)
		}
		withSecrets := rc.withSecrets
		if err := performMFAIfNeeded(withSecrets && slices.Contains(mfaKinds, sr.Kind)); err != nil {
			return trace.Wrap(err)
		}
		collection, err := rc.getCollectionByScopedRef(ctx, client, sr, resources.GetOpts{WithSecrets: withSecrets})
		if err != nil {
			return trace.Wrap(err)
		}
		return writeCollection(collection)
	}

	// No id: parse the full ref string, which may be "all", a comma-separated
	// list, or a single kind[/subkind[/name]].
	refs, err := services.ParseRefs(rc.ref)
	if err != nil {
		return trace.Wrap(err)
	}
	withSecrets := rc.withSecrets || slices.ContainsFunc(refs, func(r services.Ref) bool {
		return r.Kind == types.KindToken // tokens cannot be retrieved without secrets
	})
	mfaRequired := withSecrets && slices.ContainsFunc(refs, func(r services.Ref) bool {
		return slices.Contains(mfaKinds, r.Kind)
	})
	if err := performMFAIfNeeded(mfaRequired); err != nil {
		return trace.Wrap(err)
	}

	if refs.IsAll() {
		return rc.GetAll(ctx, client)
	}
	if len(refs) != 1 {
		return rc.getMany(ctx, client, false, refs)
	}
	collection, err := rc.getCollectionByRef(ctx, client, refs[0], resources.GetOpts{WithSecrets: withSecrets})
	if err != nil {
		return trace.Wrap(err)
	}
	return writeCollection(collection)
}

func (rc *ResourceCommand) getMany(
	ctx context.Context,
	client *authclient.Client,
	skipNotSupported bool,
	refs services.Refs,
) error {
	if rc.format != teleport.YAML {
		return trace.BadParameter("mixed resource types only support YAML formatting")
	}

	opts := resources.GetOpts{WithSecrets: rc.withSecrets}
	var allResources []types.Resource
	for _, ref := range refs {
		collection, err := rc.getCollectionByRef(ctx, client, ref, opts)
		if skipNotSupported && errors.As(err, new(*errNotSupported)) {
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}
		allResources = append(allResources, collection.Resources()...)
	}
	if err := utils.WriteYAML(rc.Stdout, allResources); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) GetAll(ctx context.Context, client *authclient.Client) error {
	rc.withSecrets = true
	allKinds := services.GetResourceMarshalerKinds()
	allRefs := make(services.Refs, 0, len(allKinds))
	for _, kind := range allKinds {
		allRefs = append(allRefs, services.Ref{Kind: kind})
	}

	// This lets OSS query Enterprise-only kinds without failing when the
	// corresponding RPCs return "NotImplemented".
	const skipNotSupported = true

	return rc.getMany(ctx, client, skipNotSupported, allRefs)
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

		opts := resources.CreateOpts{
			Force:   rc.force,
			Confirm: rc.confirm,
		}

		// Try looking for a resource handler
		if resourceHandler, found := resources.Handlers()[raw.Kind]; found {
			// only return in case of error, to create multiple resources
			// in case if yaml spec is a list
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

		// Try looking for a scoped resource handler
		if scopedHandler, found := resources.ScopedHandlers()[raw.Kind]; found {
			if err := scopedHandler.Create(ctx, client, raw, opts); err != nil {
				if trace.IsAlreadyExists(err) {
					return trace.Wrap(err, "use -f or --force flag to overwrite")
				}
				if trace.IsNotImplemented(err) {
					return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
				}
				return trace.Wrap(err)
			}
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
		_, err = client.DevicesClient().UpsertDevice(ctx, devicepb.UpsertDeviceRequest_builder{
			Device:           dev,
			CreateAsResource: true,
		}.Build())
		// err checked below
	} else {
		_, err = client.DevicesClient().CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device:           dev,
			CreateAsResource: true,
		}.Build())
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
		dev.GetAssetTag(),
		devicetrust.FriendlyOSType(dev.GetOsType()),
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
	sr, err := ParseScopedRef(rc.ref, rc.id)
	if err != nil {
		return trace.Wrap(err)
	}

	// Connectors are a special case. As it's the only meta-resource we have,
	// it's easier to special-case it here instead of adding a case in the
	// generic [resources.Handler].
	if sr.Kind == types.KindConnectors {
		return trace.BadParameter(
			"Deleting connector resources requires using an explicit connector type. Please try again with the appropriate type: %s",
			[]string{
				types.KindGithubConnector + "/" + sr.Name,
				types.KindOIDCConnector + "/" + sr.Name,
				types.KindSAMLConnector + "/" + sr.Name,
			},
		)
	}

	if sr.Scope != "" {
		handler, found := resources.ScopedHandlers()[sr.Kind]
		if !found {
			return trace.BadParameter("resource type %q does not support scope-qualified names", sr.Kind)
		}
		return trace.Wrap(handler.Delete(ctx, client, sr.SubKind, sr.SQN()))
	}

	// if scope was not set, we're about to interact with a bunch of functions that expect
	// an explicitly unscoped ref.
	ref := sr.Ref()

	// Try looking for a resource handler
	if resourceHandler, found := resources.Handlers()[ref.Kind]; found {
		if err := resourceHandler.Delete(ctx, client, ref); err != nil {
			if trace.IsNotImplemented(err) {
				return trace.BadParameter("deleting resources of type %q is not supported", ref.Kind)
			}
			return trace.Wrap(err, "error deleting resource %q of type %q", ref.Name, ref.Kind)
		}
		return nil
	}

	// check if this kind has a scoped handler. note that we check this *after* checking for an unscoped handler. resource
	// kinds can be "double registered" as both a scoped and unscoped handler if/when we want to support a mix of scoped and
	// unscoped interaction with the resource, so a missing scope isn't necessarily an error until we confirm that there
	// isn't a suitable unscoped handler.
	if _, found := resources.ScopedHandlers()[ref.Kind]; found {
		kindSpec := ref.Kind
		if ref.SubKind != "" {
			kindSpec = ref.Kind + "/" + ref.SubKind
		}
		if sr.Name != "" {
			return trace.BadParameter("resource type %q requires a scope-qualified name: tctl rm %s <scope>::%s", ref.Kind, kindSpec, sr.Name)
		}
		return trace.BadParameter("use 'tctl rm %s <scope>::<name>' to delete a %s", kindSpec, ref.Kind)
	}

	// Else fallback to the legacy logic
	singletonResources := []string{
		types.KindSessionRecordingConfig,
		types.KindInstaller,
		types.KindUIConfig,
		types.KindNetworkRestrictions,
	}
	if !slices.Contains(singletonResources, ref.Kind) && (ref.Kind == "" || ref.Name == "") {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}

	switch ref.Kind {
	case types.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", ref.Name)
	case types.KindTrustedCluster:
		if err = client.DeleteTrustedCluster(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", ref.Name)
	case types.KindRemoteCluster:
		if err = client.DeleteRemoteCluster(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("remote cluster %q has been deleted\n", ref.Name)
	case types.KindSemaphore:
		if ref.SubKind == "" || ref.Name == "" {
			return trace.BadParameter(
				"full semaphore path must be specified (e.g. '%s/%s/alice@example.com')",
				types.KindSemaphore, types.SemaphoreKindConnection,
			)
		}
		err := client.DeleteSemaphore(ctx, types.SemaphoreFilter{
			SemaphoreKind: ref.SubKind,
			SemaphoreName: ref.Name,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("semaphore '%s/%s' has been deleted\n", ref.SubKind, ref.Name)
	case types.KindDatabaseServer:
		servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		resDesc := "database server"
		servers = resources.FilterByNameOrDiscoveredName(servers, ref.Name)
		name, err := resources.GetOneResourceNameToDelete(servers, ref, resDesc)
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
		if err := client.CrownJewelsClient().DeleteCrownJewel(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown_jewel %q has been deleted\n", ref.Name)
	case types.KindDevice:
		remote := client.DevicesClient()
		device, err := findDeviceByIDOrTag(ctx, remote, ref.Name)
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := remote.DeleteDevice(ctx, devicepb.DeleteDeviceRequest_builder{
			DeviceId: device[0].GetId(),
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Device %q removed\n", ref.Name)

	case types.KindIntegration:
		if err := client.DeleteIntegration(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Integration %q removed\n", ref.Name)
	case types.KindOktaAssignment:
		if err := client.OktaClient().DeleteOktaAssignment(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Okta assignment %q has been deleted\n", ref.Name)
	case types.KindOktaImportRule:
		if err := client.OktaClient().DeleteOktaImportRule(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Okta import rule %q has been deleted\n", ref.Name)
	case types.KindUserGroup:
		if err := client.DeleteUserGroup(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("User group %q has been deleted\n", ref.Name)
	case types.KindSecurityReport:
		if err := client.SecReportsClient().DeleteSecurityReport(ctx, ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Security report %q has been deleted\n", ref.Name)
	case types.KindHealthCheckConfig:
		return trace.Wrap(rc.deleteHealthCheckConfig(ctx, client, ref.Name))
	default:
		return trace.BadParameter("deleting resources of type %q is not supported", ref.Kind)
	}
	return nil
}

func resetNetworkRestrictions(ctx context.Context, client *authclient.Client) error {
	return trace.Wrap(client.DeleteNetworkRestrictions(ctx))
}

// UpdateFields updates select resource fields: expiry and labels
func (rc *ResourceCommand) UpdateFields(ctx context.Context, clt *authclient.Client) error {
	sr, err := ParseScopedRef(rc.ref, rc.id)
	if err != nil {
		return trace.Wrap(err)
	}
	if sr.Kind == "" || sr.Name == "" {
		return trace.BadParameter("provide a full resource name to update, for example:\n$ tctl update rc/remote --set-labels=env=prod\n")
	}
	if sr.Scope != "" {
		return trace.BadParameter("resource type %q does not support scope-qualified names", sr.Kind)
	}

	var labels map[string]string
	if rc.labels != "" {
		labels, err = parse.LabelSelectorSpec(rc.labels)
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

	switch sr.Kind {
	case types.KindRemoteCluster:
		cluster, err := clt.GetRemoteCluster(ctx, sr.Name)
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
		fmt.Printf("cluster %v has been updated\n", sr.Name)
	default:
		return trace.BadParameter("updating resources of type %q is not supported, supported are: %q", sr.Kind, types.KindRemoteCluster)
	}
	return nil
}

// IsForced returns true if -f flag was passed
func (rc *ResourceCommand) IsForced() bool {
	return rc.force
}

func (rc *ResourceCommand) getCollectionByScopedRef(ctx context.Context, client *authclient.Client, sr ScopedRef, opts resources.GetOpts) (resources.Collection, error) {
	if sr.Scope != "" {
		handler, found := resources.ScopedHandlers()[sr.Kind]
		if !found {
			return nil, trace.BadParameter("resource type %q does not support scope-qualified names", sr.Kind)
		}
		sqn := sr.SQN()
		return handler.Get(ctx, client, sr.SubKind, &sqn, opts)
	}

	// get was invoked without a scope being specified. this is valid for unscoped types, or when doing a list operation
	// for a scoped type. some types are registered as both scoped and unscoped. therefore we specifically want to
	// reject attempts to invoke a single-resource get at this point only if it targets a type that *does* have a scoped
	// handler and *does not* have an unscoped handler.
	if sr.Name != "" {
		_, classicFound := resources.Handlers()[sr.Kind]
		_, scopedFound := resources.ScopedHandlers()[sr.Kind]
		if !classicFound && scopedFound {
			kindSpec := sr.Kind
			if sr.SubKind != "" {
				kindSpec = fmt.Sprintf("%s/%s", sr.Kind, sr.SubKind)
			}
			return nil, trace.BadParameter(
				"resource type %q requires a scope-qualified name, e.g.:\n  tctl get %s <scope>::%s",
				kindSpec, kindSpec, sr.Name,
			)
		}
	}

	// fallthrough to the general getCollectionByRef logic. getCollectionByRef handles unscoped types, but it also handles
	// invocations for scoped list operations.
	return rc.getCollectionByRef(ctx, client, sr.Ref(), opts)
}

func (rc *ResourceCommand) getCollectionByRef(ctx context.Context, client *authclient.Client, ref services.Ref, opts resources.GetOpts) (resources.Collection, error) {
	if ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}

	if handler, found := resources.Handlers()[ref.Kind]; found {
		coll, err := handler.Get(ctx, client, ref, opts)
		if err != nil {
			if trace.IsNotImplemented(err) {
				return nil, &errNotSupported{trace.BadParameter("getting %q is not supported", ref.String())}
			}
			return nil, trace.Wrap(err, "getting resource %q of type %q", ref.Name, ref.Kind)
		}
		return coll, nil
	}

	// Scoped handler path: list-all only. Single-resource lookup for purely-scoped
	// kinds requires a scope-qualified name, handled by getCollectionByScopedRef.
	if handler, found := resources.ScopedHandlers()[ref.Kind]; found {
		if ref.Name != "" {
			kindSpec := ref.Kind
			if ref.SubKind != "" {
				kindSpec = fmt.Sprintf("%s/%s", ref.Kind, ref.SubKind)
			}
			return nil, trace.BadParameter(
				"resource type %q requires a scope-qualified name, e.g.:\n  tctl get %s <scope>::%s\nuse 'tctl get %s' to list all resources of this type",
				kindSpec, kindSpec, ref.Name, ref.Kind,
			)
		}
		if ref.SubKind != "" {
			// technically our current CLI syntax does not support specifying a sub-kind without specifying
			// a name so this should currently be unreachable, but its a good defensive check since hander.Get
			// will likely not know how to handle sub-kind listing if future changes make it possible to express.
			return nil, trace.BadParameter("listing resources by sub-kind is not supported, use 'tctl get %s' to list all resources of this kind", ref.Kind)
		}
		return handler.Get(ctx, client, "", nil, opts)
	}

	// The resource hasn't been migrated yet, falling back to the old logic.

	switch ref.Kind {
	case types.KindReverseTunnel:
		if ref.Name != "" {
			return nil, trace.BadParameter("reverse tunnel cannot be searched by name")
		}

		tunnels, err := stream.Collect(clientutils.Resources(ctx, client.ListReverseTunnels))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &reverseTunnelCollection{tunnels: tunnels}, nil
	case types.KindTrustedCluster:
		if ref.Name == "" {
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
		trustedCluster, err := client.GetTrustedCluster(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []types.TrustedCluster{trustedCluster}}, nil
	case types.KindRemoteCluster:
		if ref.Name == "" {
			remoteClusters, err := client.GetRemoteClusters(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &remoteClusterCollection{remoteClusters: remoteClusters}, nil
		}
		remoteCluster, err := client.GetRemoteCluster(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &remoteClusterCollection{remoteClusters: []types.RemoteCluster{remoteCluster}}, nil
	case types.KindSemaphore:
		filter := types.SemaphoreFilter{
			SemaphoreKind: ref.SubKind,
			SemaphoreName: ref.Name,
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
		if ref.Name == "" {
			return &databaseServerCollection{servers: servers}, nil
		}

		servers = resources.FilterByNameOrDiscoveredName(servers, ref.Name)
		if len(servers) == 0 {
			return nil, trace.NotFound("database server %q not found", ref.Name)
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
		resourceName := ref.Name
		listReq := proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
		}
		if resourceName != "" {
			listReq.PredicateExpression = fmt.Sprintf(`name == %q`, resourceName)
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
	case types.KindDevice:
		remote := client.DevicesClient()
		if ref.Name != "" {
			resp, err := remote.FindDevices(ctx, devicepb.FindDevicesRequest_builder{
				IdOrTag: ref.Name,
			}.Build())
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &deviceCollection{resp.GetDevices()}, nil
		}

		req := devicepb.ListDevicesRequest_builder{
			View: devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		}.Build()
		var devs []*devicepb.Device
		for {
			resp, err := remote.ListDevices(ctx, req)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			devs = append(devs, resp.GetDevices()...)

			if resp.GetNextPageToken() == "" {
				break
			}
			req.SetPageToken(resp.GetNextPageToken())
		}

		sort.Slice(devs, func(i, j int) bool {
			d1 := devs[i]
			d2 := devs[j]

			if d1.GetAssetTag() == d2.GetAssetTag() {
				return d1.GetOsType() < d2.GetOsType()
			}

			return d1.GetAssetTag() < d2.GetAssetTag()
		})

		return &deviceCollection{devices: devs}, nil
	case types.KindOktaImportRule:
		if ref.Name != "" {
			importRule, err := client.OktaClient().GetOktaImportRule(ctx, ref.Name)
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
		if ref.Name != "" {
			assignment, err := client.OktaClient().GetOktaAssignment(ctx, ref.Name)
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
		if ref.Name != "" {
			userGroup, err := client.GetUserGroup(ctx, ref.Name)
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
	case types.KindIntegration:
		if ref.Name != "" {
			ig, err := client.GetIntegration(ctx, ref.Name)
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
		if ref.Name != "" {

			resource, err := client.SecReportsClient().GetSecurityReport(ctx, ref.Name)
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
	case types.KindPlugin:
		if ref.Name != "" {
			plugin, err := client.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: ref.Name}.Build())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &pluginCollection{plugins: []types.Plugin{plugin}}, nil
		}
		var plugins []types.Plugin
		startKey := ""
		for {
			resp, err := client.PluginsClient().ListPlugins(ctx, pluginsv1.ListPluginsRequest_builder{
				PageSize:    100,
				StartKey:    startKey,
				WithSecrets: rc.withSecrets,
			}.Build())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			for _, v := range resp.GetPlugins() {
				plugins = append(plugins, v)
			}
			if resp.GetNextKey() == "" {
				break
			}
			startKey = resp.GetNextKey()
		}
		return &pluginCollection{plugins: plugins}, nil

	case types.KindHealthCheckConfig:
		if ref.Name != "" {
			cfg, err := client.GetHealthCheckConfig(ctx, ref.Name)
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
	}
	return nil, trace.BadParameter("getting %q is not supported", ref.String())
}

// errNotSupported is used to mark NotImplemented errors that were transformed
// into BadParameter, so they can later be identified.
type errNotSupported struct {
	cause error
}

func (e *errNotSupported) Error() string {
	return e.cause.Error()
}

func (e *errNotSupported) Unwrap() error {
	return e.cause
}

// UpsertVerb generates the correct string form of a verb based on the action taken
func UpsertVerb(exists bool, force bool) string {
	if !force && exists {
		return "updated"
	}
	return "created"
}

func findDeviceByIDOrTag(ctx context.Context, remote devicepb.DeviceTrustServiceClient, idOrTag string) ([]*devicepb.Device, error) {
	resp, err := remote.FindDevices(ctx, devicepb.FindDevicesRequest_builder{
		IdOrTag: idOrTag,
	}.Build())
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case len(resp.GetDevices()) == 0:
		return nil, trace.NotFound("device %q not found", idOrTag)
	case len(resp.GetDevices()) == 1:
		return resp.GetDevices(), nil
	}

	// Do we have an ID match?
	for _, dev := range resp.GetDevices() {
		if dev.GetId() == idOrTag {
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

func (rc *ResourceCommand) updatePlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	item := pluginResourceWrapper{PluginV1: types.PluginV1{}}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, pluginsv1.UpdatePluginRequest_builder{Plugin: &item.PluginV1}.Build()); err != nil {
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
	if _, err := client.PluginsClient().UpdatePlugin(ctx, pluginsv1.UpdatePluginRequest_builder{Plugin: &item.PluginV1}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("plugin %q has been updated\n", item.GetName())
	return nil
}

// UpdateFields updates select resource fields: expiry and labels
func (rc *ResourceCommand) listKinds() error {
	rows := resourceKindRows()

	switch rc.format {
	case teleport.Text:
		return trace.Wrap(rc.writeResourceKindRowsText(rows))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(rc.Stdout, rows))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAMLArray(rc.Stdout, rows))
	default:
		return trace.BadParameter("unsupported format %q", rc.format)
	}
}

type resourceKindRow struct {
	Kind              string   `json:"kind"`
	SupportedCommands []string `json:"supported_commands"`
	Singleton         bool     `json:"singleton"`
	MFARequired       bool     `json:"mfa_required"`
	Description       string   `json:"description"`
}

func resourceKindRows() []resourceKindRow {
	rows := make([]resourceKindRow, 0, len(resources.Handlers()))
	for kind, handler := range resources.Handlers() {
		rows = append(rows, resourceKindRow{
			Kind:              kind,
			SupportedCommands: handler.SupportedCommands(),
			Singleton:         handler.Singleton(),
			MFARequired:       handler.MFARequired(),
			Description:       handler.Description(),
		})
	}
	for kind, handler := range resources.ScopedHandlers() {
		rows = append(rows, resourceKindRow{
			Kind:              kind,
			SupportedCommands: handler.SupportedCommands(),
			Singleton:         false, // scoped resources are never singletons
			MFARequired:       handler.MFARequired(),
			Description:       handler.Description(),
		})
	}
	slices.SortStableFunc(rows, func(a, b resourceKindRow) int {
		return strings.Compare(a.Kind, b.Kind)
	})
	return rows
}

func (rc *ResourceCommand) writeResourceKindRowsText(kindRows []resourceKindRow) error {
	// We must compute rows before, and cannot add them as we go
	// because this breaks the "truncated columns behavior"
	rows := make([][]string, 0, len(kindRows))
	for _, row := range kindRows {
		rows = append(rows, []string{
			row.Kind,
			strings.Join(row.SupportedCommands, ","),
			yesOrEmpty(row.Singleton),
			yesOrEmpty(row.MFARequired),
			row.Description,
		})
	}
	var t asciitable.Table
	headers := []string{"Kind", "Supported Commands", "Singleton", "MFA", "Description"}
	if rc.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}
	return trace.Wrap(t.WriteTo(rc.Stdout))
}

func yesOrEmpty(b bool) string {
	if b {
		return "yes"
	}
	return ""
}
