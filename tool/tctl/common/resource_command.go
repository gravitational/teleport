/*
Copyright 2015-2019 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// ResourceCreateHandler is the generic implementation of a resource creation handler
type ResourceCreateHandler func(auth.ClientI, services.UnknownResource) error

// ResourceKind is the string form of a resource, i.e. "oidc"
type ResourceKind string

// ResourceCommand implements `tctl get/create/list` commands for manipulating
// Teleport resources
type ResourceCommand struct {
	config      *service.Config
	ref         services.Ref
	refs        services.Refs
	format      string
	namespace   string
	withSecrets bool
	force       bool

	// filename is the name of the resource, used for 'create'
	filename string

	// CLI subcommands:
	deleteCmd *kingpin.CmdClause
	getCmd    *kingpin.CmdClause
	createCmd *kingpin.CmdClause

	CreateHandlers map[ResourceKind]ResourceCreateHandler
}

const getHelp = `Examples:

  $ tctl get clusters       : prints the list of all trusted clusters
  $ tctl get cluster/east   : prints the trusted cluster 'east'
  $ tctl get clusters,users : prints all trusted clusters and all users

Same as above, but using JSON output:

  $ tctl get clusters --format=json
`

// Initialize allows ResourceCommand to plug itself into the CLI parser
func (rc *ResourceCommand) Initialize(app *kingpin.Application, config *service.Config) {
	rc.CreateHandlers = map[ResourceKind]ResourceCreateHandler{
		services.KindUser:            rc.createUser,
		services.KindTrustedCluster:  rc.createTrustedCluster,
		services.KindGithubConnector: rc.createGithubConnector,
		services.KindCertAuthority:   rc.createCertAuthority,
	}
	rc.config = config

	rc.createCmd = app.Command("create", "Create or update a Teleport resource from a YAML file")
	rc.createCmd.Arg("filename", "resource definition file").Required().StringVar(&rc.filename)
	rc.createCmd.Flag("force", "Overwrite the resource if already exists").Short('f').BoolVar(&rc.force)

	rc.deleteCmd = app.Command("rm", "Delete a resource").Alias("del")
	rc.deleteCmd.Arg("resource type/resource name", `Resource to delete
	<resource type>  Type of a resource [for example: connector,user,cluster,token]
	<resource name>  Resource name to delete

	Examples:
	$ tctl rm connector/github
	$ tctl rm cluster/main`).SetValue(&rc.ref)

	rc.getCmd = app.Command("get", "Print a YAML declaration of various Teleport resources")
	rc.getCmd.Arg("resources", "Resource spec: 'type/[name][,...]' or 'all'").Required().SetValue(&rc.refs)
	rc.getCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&rc.format)
	rc.getCmd.Flag("namespace", "Namespace of the resources").Hidden().Default(defaults.Namespace).StringVar(&rc.namespace)
	rc.getCmd.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&rc.withSecrets)

	rc.getCmd.Alias(getHelp)
}

// TryRun takes the CLI command as an argument (like "auth gen") and executes it
// or returns match=false if 'cmd' does not belong to it
func (rc *ResourceCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	// tctl get
	case rc.getCmd.FullCommand():
		err = rc.Get(client)
		// tctl create
	case rc.createCmd.FullCommand():
		err = rc.Create(client)
		// tctl rm
	case rc.deleteCmd.FullCommand():
		err = rc.Delete(client)
	default:
		return false, nil
	}
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
func (rc *ResourceCommand) Get(client auth.ClientI) error {
	if rc.refs.IsAll() {
		return rc.GetAll(client)
	}
	if len(rc.refs) != 1 {
		return rc.GetMany(client)
	}
	rc.ref = rc.refs[0]
	collection, err := rc.getCollection(client)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note that only YAML is officially supported. Support for text and JSON
	// is experimental.
	switch rc.format {
	case teleport.YAML:
		return collection.writeYAML(os.Stdout)
	case teleport.Text:
		return collection.writeText(os.Stdout)
	case teleport.JSON:
		return collection.writeJSON(os.Stdout)
	}
	return trace.BadParameter("unsupported format")
}

func (rc *ResourceCommand) GetMany(client auth.ClientI) error {
	if rc.format != teleport.YAML {
		return trace.BadParameter("mixed resource types only support YAML formatting")
	}
	var resources []services.Resource
	for _, ref := range rc.refs {
		rc.ref = ref
		collection, err := rc.getCollection(client)
		if err != nil {
			return trace.Wrap(err)
		}
		resources = append(resources, collection.resources()...)
	}
	if err := utils.WriteYAML(os.Stdout, resources); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) GetAll(client auth.ClientI) error {
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
	return rc.GetMany(client)
}

// Create updates or inserts one or many resources
func (rc *ResourceCommand) Create(client auth.ClientI) error {
	reader, err := utils.OpenFile(rc.filename)
	if err != nil {
		return trace.Wrap(err)
	}
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	count := 0
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				if count == 0 {
					return trace.BadParameter("no resources found, empty input?")
				}
				return nil
			}
			return trace.Wrap(err)
		}
		count++

		// locate the creator function for a given resource kind:
		creator, found := rc.CreateHandlers[ResourceKind(raw.Kind)]
		if !found {
			// if we're trying to create an OIDC/SAML connector with the OSS version of tctl, return a specific error
			if raw.Kind == "oidc" || raw.Kind == "saml" {
				return trace.BadParameter("creating resources of type %q is only supported in Teleport Enterprise. https://gravitational.com/teleport/docs/enterprise/", raw.Kind)
			}
			return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
		}
		// only return in case of error, to create multiple resources
		// in case if yaml spec is a list
		if err := creator(client, raw); err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.Wrap(err, "use -f or --force flag to overwrite")
			}
			return trace.Wrap(err)
		}
	}
}

// createTrustedCluster implements `tctl create cluster.yaml` command
func (rc *ResourceCommand) createTrustedCluster(client auth.ClientI, raw services.UnknownResource) error {
	tc, err := services.GetTrustedClusterMarshaler().Unmarshal(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	// check if such cluster already exists:
	name := tc.GetName()
	_, err = client.GetTrustedCluster(name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !rc.force && exists {
		return trace.AlreadyExists("trusted cluster %q already exists", name)
	}

	out, err := client.UpsertTrustedCluster(context.TODO(), tc)
	if err != nil {
		// If force is used and UpsertTrustedCluster returns trace.AlreadyExists,
		// this means the user tried to upsert a cluster whose exact match already
		// exists in the backend, nothing needs to occur other than happy message
		// that the trusted cluster has been created.
		if rc.force && trace.IsAlreadyExists(err) {
			out = tc
		} else {
			return trace.Wrap(err)
		}
	}
	if out.GetName() != tc.GetName() {
		fmt.Printf("WARNING: trusted cluster %q resource has been renamed to match remote cluster name %q\n", name, out.GetName())
	}
	fmt.Printf("trusted cluster %q has been %v\n", out.GetName(), UpsertVerb(exists, rc.force))
	return nil
}

// createCertAuthority creates certificate authority
func (rc *ResourceCommand) createCertAuthority(client auth.ClientI, raw services.UnknownResource) error {
	certAuthority, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.UpsertCertAuthority(certAuthority); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("certificate authority '%s' has been updated\n", certAuthority.GetName())
	return nil
}

// createGithubConnector creates a Github connector
func (rc *ResourceCommand) createGithubConnector(client auth.ClientI, raw services.UnknownResource) error {
	connector, err := services.GetGithubConnectorMarshaler().Unmarshal(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.GetGithubConnector(connector.GetName(), false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)
	if !rc.force && exists {
		return trace.AlreadyExists("authentication connector %q already exists",
			connector.GetName())
	}
	err = client.UpsertGithubConnector(context.TODO(), connector)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been %s\n",
		connector.GetName(), UpsertVerb(exists, rc.force))
	return nil
}

// createUser implements 'tctl create user.yaml' command
func (rc *ResourceCommand) createUser(client auth.ClientI, raw services.UnknownResource) error {
	user, err := services.GetUserMarshaler().UnmarshalUser(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	userName := user.GetName()
	_, err = client.GetUser(userName, false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !rc.force {
			return trace.AlreadyExists("user %q already exists", userName)
		}

		if err := client.UpdateUser(context.TODO(), user); err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("user %q has been updated\n", userName)
	} else {
		if err := client.CreateUser(context.TODO(), user); err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("user %q has been created\n", userName)
	}

	return nil
}

// Delete deletes resource by name
func (rc *ResourceCommand) Delete(client auth.ClientI) (err error) {
	if rc.ref.Kind == "" || rc.ref.Name == "" {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}

	ctx := context.TODO()
	switch rc.ref.Kind {
	case services.KindNode:
		if err = client.DeleteNode(defaults.Namespace, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("node %v has been deleted\n", rc.ref.Name)
	case services.KindUser:
		if err = client.DeleteUser(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been deleted\n", rc.ref.Name)
	case services.KindSAMLConnector:
		if err = client.DeleteSAMLConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML connector %v has been deleted\n", rc.ref.Name)
	case services.KindOIDCConnector:
		if err = client.DeleteOIDCConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("OIDC connector %v has been deleted\n", rc.ref.Name)
	case services.KindGithubConnector:
		if err = client.DeleteGithubConnector(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("github connector %q has been deleted\n", rc.ref.Name)
	case services.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", rc.ref.Name)
	case services.KindTrustedCluster:
		if err = client.DeleteTrustedCluster(ctx, rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", rc.ref.Name)
	case services.KindRemoteCluster:
		if err = client.DeleteRemoteCluster(rc.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("remote cluster %q has been deleted\n", rc.ref.Name)
	default:
		return trace.BadParameter("deleting resources of type %q is not supported", rc.ref.Kind)
	}
	return nil
}

// IsForced returns true if -f flag was passed
func (rc *ResourceCommand) IsForced() bool {
	return rc.force
}

// getCollection lists all resources of a given type
func (rc *ResourceCommand) getCollection(client auth.ClientI) (c ResourceCollection, err error) {
	if rc.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch rc.ref.Kind {
	// load user(s)
	case services.KindUser:
		var users services.Users
		// just one?
		if !rc.ref.IsEmpty() {
			user, err := client.GetUser(rc.ref.Name, rc.withSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			users = services.Users{user}
			// all of them?
		} else {
			users, err = client.GetUsers(rc.withSecrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return &userCollection{users: users}, nil
	case services.KindConnectors:
		sc, err := client.GetSAMLConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		oc, err := client.GetOIDCConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		gc, err := client.GetGithubConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &connectorsCollection{
			saml:   sc,
			oidc:   oc,
			github: gc,
		}, nil
	case services.KindSAMLConnector:
		connectors, err := client.GetSAMLConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors: connectors}, nil
	case services.KindOIDCConnector:
		connectors, err := client.GetOIDCConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors: connectors}, nil
	case services.KindGithubConnector:
		connectors, err := client.GetGithubConnectors(rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &githubCollection{connectors: connectors}, nil
	case services.KindReverseTunnel:
		tunnels, err := client.GetReverseTunnels()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &reverseTunnelCollection{tunnels: tunnels}, nil
	case services.KindCertAuthority:
		userAuthorities, err := client.GetCertAuthorities(services.UserCA, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hostAuthorities, err := client.GetCertAuthorities(services.HostCA, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAuthorities = append(userAuthorities, hostAuthorities...)
		return &authorityCollection{cas: userAuthorities}, nil
	case services.KindNode:
		nodes, err := client.GetNodes(rc.namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: nodes}, nil
	case services.KindAuthServer:
		servers, err := client.GetAuthServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindProxy:
		servers, err := client.GetProxies()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindRole:
		if rc.ref.Name == "" {
			roles, err := client.GetRoles()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &roleCollection{roles: roles}, nil
		}
		role, err := client.GetRole(rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: []services.Role{role}}, nil
	case services.KindNamespace:
		if rc.ref.Name == "" {
			namespaces, err := client.GetNamespaces()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &namespaceCollection{namespaces: namespaces}, nil
		}
		ns, err := client.GetNamespace(rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &namespaceCollection{namespaces: []services.Namespace{*ns}}, nil
	case services.KindTrustedCluster:
		if rc.ref.Name == "" {
			trustedClusters, err := client.GetTrustedClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
		}
		trustedCluster, err := client.GetTrustedCluster(rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []services.TrustedCluster{trustedCluster}}, nil
	case services.KindRemoteCluster:
		if rc.ref.Name == "" {
			remoteClusters, err := client.GetRemoteClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &remoteClusterCollection{remoteClusters: remoteClusters}, nil
		}
		remoteCluster, err := client.GetRemoteCluster(rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &remoteClusterCollection{remoteClusters: []services.RemoteCluster{remoteCluster}}, nil
	}
	return nil, trace.BadParameter("'%v' is not supported", rc.ref.Kind)
}

// UpsertVerb generates the correct string form of a verb based on the action taken
func UpsertVerb(exists bool, force bool) string {
	switch {
	case exists == true && force == true:
		return "created"
	case exists == false && force == true:
		return "created"
	case exists == true && force == false:
		return "updated"
	case exists == false && force == false:
		return "created"
	default:
		// Unreachable, but compiler requires this.
		return "unknown"
	}
}
