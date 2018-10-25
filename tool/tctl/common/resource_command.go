/*
Copyright 2015-2017 Gravitational, Inc.

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
	"fmt"
	"io"
	"os"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type ResourceCreateHandler func(auth.ClientI, services.UnknownResource) error
type ResourceKind string

// ResourceCommand implements `tctl get/create/list` commands for manipulating
// Teleport resources
type ResourceCommand struct {
	config      *service.Config
	ref         services.Ref
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

  $ tctl get clusters      : prints the list of all trusted clusters
  $ tctl get cluster/east  : prints the trusted cluster 'east'

Same as above, but using JSON output:

  $ tctl get clusters --format=json
`

// Initialize allows ResourceCommand to plug itself into the CLI parser
func (g *ResourceCommand) Initialize(app *kingpin.Application, config *service.Config) {
	g.CreateHandlers = map[ResourceKind]ResourceCreateHandler{
		services.KindUser:            g.createUser,
		services.KindTrustedCluster:  g.createTrustedCluster,
		services.KindGithubConnector: g.createGithubConnector,
		services.KindCertAuthority:   g.createCertAuthority,
	}
	g.config = config

	g.createCmd = app.Command("create", "Create or update a Teleport resource from a YAML file")
	g.createCmd.Arg("filename", "resource definition file").Required().StringVar(&g.filename)
	g.createCmd.Flag("force", "Overwrite the resource if already exists").Short('f').BoolVar(&g.force)

	g.deleteCmd = app.Command("rm", "Delete a resource").Alias("del")
	g.deleteCmd.Arg("resource", "Resource to delete").SetValue(&g.ref)

	g.getCmd = app.Command("get", "Print a YAML declaration of various Teleport resources")
	g.getCmd.Arg("resource", "Resource spec: 'type/[name]'").SetValue(&g.ref)
	g.getCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(formatYAML).StringVar(&g.format)
	g.getCmd.Flag("namespace", "Namespace of the resources").Hidden().Default(defaults.Namespace).StringVar(&g.namespace)
	g.getCmd.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&g.withSecrets)

	g.getCmd.Alias(getHelp)
}

// TryRun takes the CLI command as an argument (like "auth gen") and executes it
// or returns match=false if 'cmd' does not belong to it
func (g *ResourceCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	// tctl get
	case g.getCmd.FullCommand():
		err = g.Get(client)
		// tctl create
	case g.createCmd.FullCommand():
		err = g.Create(client)
		// tctl rm
	case g.deleteCmd.FullCommand():
		err = g.Delete(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// IsDeleteSubcommand returns 'true' if the given command is `tctl rm`
func (g *ResourceCommand) IsDeleteSubcommand(cmd string) bool {
	return cmd == g.deleteCmd.FullCommand()
}

// GetRef returns the reference (basically type/name pair) of the resource
// the command is operating on
func (g *ResourceCommand) GetRef() services.Ref {
	return g.ref
}

// Get prints one or many resources of a certain type
func (g *ResourceCommand) Get(client auth.ClientI) error {
	collection, err := g.getCollection(client)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note that only YAML is officially supported. Support for text and JSON
	// is experimental.
	switch g.format {
	case formatYAML:
		return collection.writeYAML(os.Stdout)
	case formatText:
		return collection.writeText(os.Stdout)
	case formatJSON:
		return collection.writeJSON(os.Stdout)
	}
	return trace.BadParameter("unsupported format")
}

// Create updates or insterts one or many resources
func (u *ResourceCommand) Create(client auth.ClientI) error {
	reader, err := utils.OpenFile(u.filename)
	if err != nil {
		return trace.Wrap(err)
	}
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
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
		count += 1

		// locate the creator function for a given resource kind:
		creator, found := u.CreateHandlers[ResourceKind(raw.Kind)]
		if !found {
			return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
		}
		// only return in case of error, to create multiple resources
		// in case if yaml spec is a list
		if err := creator(client, raw); err != nil {
			return trace.Wrap(err)
		}
	}
}

// createTrustedCluster implements `tctl create cluster.yaml` command
func (u *ResourceCommand) createTrustedCluster(client auth.ClientI, raw services.UnknownResource) error {
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
	if u.force == false && exists {
		return trace.AlreadyExists("trusted cluster '%s' already exists", name)
	}

	out, err := client.UpsertTrustedCluster(tc)
	if err != nil {
		// If force is used and UpsertTrustedCluster returns trace.AlreadyExists,
		// this means the user tried to upsert a cluster whose exact match already
		// exists in the backend, nothing needs to occur other than happy message
		// that the trusted cluster has been created.
		if u.force && trace.IsAlreadyExists(err) {
			out = tc
		} else {
			return trace.Wrap(err)
		}
	}
	if out.GetName() != tc.GetName() {
		fmt.Printf("WARNING: trusted cluster %q resource has been renamed to match remote cluster name %q\n", name, out.GetName())
	}
	fmt.Printf("trusted cluster %q has been %v\n", out.GetName(), UpsertVerb(exists, u.force))
	return nil
}

// createCertAuthority creates certificate authority
func (u *ResourceCommand) createCertAuthority(client auth.ClientI, raw services.UnknownResource) error {
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

func (u *ResourceCommand) createGithubConnector(client auth.ClientI, raw services.UnknownResource) error {
	connector, err := services.GetGithubConnectorMarshaler().Unmarshal(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.GetGithubConnector(connector.GetName(), false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)
	if u.force == false && exists {
		return trace.AlreadyExists("authentication connector %q already exists",
			connector.GetName())
	}
	err = client.UpsertGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been %s\n",
		connector.GetName(), UpsertVerb(exists, u.force))
	return nil
}

// createUser implements 'tctl create user.yaml' command
func (u *ResourceCommand) createUser(client auth.ClientI, raw services.UnknownResource) error {
	user, err := services.GetUserMarshaler().UnmarshalUser(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	userName := user.GetName()
	if err := client.UpsertUser(user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user '%s' has been updated\n", userName)
	return nil
}

// Delete deletes resource by name
func (d *ResourceCommand) Delete(client auth.ClientI) (err error) {
	if d.ref.Kind == "" || d.ref.Name == "" {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}

	switch d.ref.Kind {
	case services.KindUser:
		if err = client.DeleteUser(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %v has been deleted\n", d.ref.Name)
	case services.KindSAMLConnector:
		if err = client.DeleteSAMLConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML Connector %v has been deleted\n", d.ref.Name)
	case services.KindOIDCConnector:
		if err = client.DeleteOIDCConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("OIDC Connector %v has been deleted\n", d.ref.Name)
	case services.KindGithubConnector:
		if err = client.DeleteGithubConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("github connector %q has been deleted\n", d.ref.Name)
	case services.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", d.ref.Name)
	case services.KindTrustedCluster:
		if err = client.DeleteTrustedCluster(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", d.ref.Name)
	case services.KindRemoteCluster:
		if err = client.DeleteRemoteCluster(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("remote cluster %q has been deleted\n", d.ref.Name)
	default:
		return trace.BadParameter("deleting resources of type %q is not supported", d.ref.Kind)
	}
	return nil
}

// IsForced returns true if -f flag was passed
func (cmd *ResourceCommand) IsForced() bool {
	return cmd.force
}

func (g *ResourceCommand) getCollection(client auth.ClientI) (c ResourceCollection, err error) {
	if g.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch g.ref.Kind {
	// load user(s)
	case services.KindUser:
		var users services.Users
		// just one?
		if !g.ref.IsEmtpy() {
			user, err := client.GetUser(g.ref.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			users = services.Users{user}
			// all of them?
		} else {
			users, err = client.GetUsers()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return &userCollection{users: users}, nil
	case services.KindConnectors:
		sc, err := client.GetSAMLConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		oc, err := client.GetOIDCConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		gc, err := client.GetGithubConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &connectorsCollection{
			saml:   sc,
			oidc:   oc,
			github: gc,
		}, nil
	case services.KindSAMLConnector:
		connectors, err := client.GetSAMLConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors: connectors}, nil
	case services.KindOIDCConnector:
		connectors, err := client.GetOIDCConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors: connectors}, nil
	case services.KindGithubConnector:
		connectors, err := client.GetGithubConnectors(g.withSecrets)
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
		userAuthorities, err := client.GetCertAuthorities(services.UserCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hostAuthorities, err := client.GetCertAuthorities(services.HostCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAuthorities = append(userAuthorities, hostAuthorities...)
		return &authorityCollection{cas: userAuthorities}, nil
	case services.KindNode:
		nodes, err := client.GetNodes(g.namespace)
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
		if g.ref.Name == "" {
			roles, err := client.GetRoles()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &roleCollection{roles: roles}, nil
		}
		role, err := client.GetRole(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: []services.Role{role}}, nil
	case services.KindNamespace:
		if g.ref.Name == "" {
			namespaces, err := client.GetNamespaces()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &namespaceCollection{namespaces: namespaces}, nil
		}
		ns, err := client.GetNamespace(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &namespaceCollection{namespaces: []services.Namespace{*ns}}, nil
	case services.KindTrustedCluster:
		if g.ref.Name == "" {
			trustedClusters, err := client.GetTrustedClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
		}
		trustedCluster, err := client.GetTrustedCluster(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []services.TrustedCluster{trustedCluster}}, nil
	case services.KindRemoteCluster:
		if g.ref.Name == "" {
			remoteClusters, err := client.GetRemoteClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &remoteClusterCollection{remoteClusters: remoteClusters}, nil
		}
		remoteCluster, err := client.GetRemoteCluster(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &remoteClusterCollection{remoteClusters: []services.RemoteCluster{remoteCluster}}, nil
	}
	return nil, trace.BadParameter("'%v' is not supported", g.ref.Kind)
}

const (
	formatYAML = "yaml"
	formatText = "text"
	formatJSON = "json"
)

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
	}

	// Can never reach here, but the compiler complains.
	return "unknown"
}
