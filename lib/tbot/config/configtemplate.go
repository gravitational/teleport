/*
Copyright 2022 Gravitational, Inc.

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

package config

import (
	"context"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const (
	// TemplateSSHClientName is the config name for generating ssh client
	// config files.
	TemplateSSHClientName = "ssh_client"

	// TemplateIdentityName is the config name for Teleport identity files.
	TemplateIdentityName = "identity"

	// TemplateTLSName is the config name for TLS client certificates.
	TemplateTLSName = "tls"

	// TemplateTLSCAsName is the config name for TLS CA certificates.
	TemplateTLSCAsName = "tls_cas"

	// TemplateMongoName is the config name for MongoDB-formatted certificates.
	TemplateMongoName = "mongo"

	// TemplateCockroachName is the config name for CockroachDB-formatted
	// certificates.
	TemplateCockroachName = "cockroach"

	// TemplateKubernetesName is the config name for generating Kubernetes
	// client config files
	TemplateKubernetesName = "kubernetes"

	// TemplateSSHHostCertName is the config name for generating SSH host
	// certificates
	TemplateSSHHostCertName = "ssh_host_cert"
)

// AllConfigTemplates lists all valid config templates, intended for help
// messages
var AllConfigTemplates = [...]string{
	TemplateSSHClientName,
	TemplateIdentityName,
	TemplateTLSName,
	TemplateTLSCAsName,
	TemplateMongoName,
	TemplateCockroachName,
	TemplateKubernetesName,
	TemplateSSHHostCertName,
}

// FileDescription is a minimal spec needed to create an empty end-user-owned
// file with bot-writable ACLs during `tbot init`.
type FileDescription struct {
	// Name is the name of the file or directory to create.
	Name string

	// IsDir designates whether this describes a subdirectory inside the
	// destination.
	IsDir bool
}

// Template defines functions for dynamically writing additional files to
// a Destination.
type Template interface {
	// Name returns the name of this config template.
	Name() string

	// Describe generates a list of all files this ConfigTemplate will generate
	// at runtime. Currently ConfigTemplates are required to know this
	// statically as this must be callable without any auth clients (or any
	// secrets) for use with `tbot init`. If an arbitrary number of files must
	// be generated, they should be placed in a subdirectory.
	Describe(destination bot.Destination) []FileDescription

	// Render writes the config template to the destination.
	Render(
		ctx context.Context,
		bot provider,
		identity *identity.Identity,
		destination *DestinationConfig,
	) error
}

// TemplateConfig contains all possible config template variants. Exactly one
// variant must be set to be considered valid.
type TemplateConfig struct {
	SSHClient   *TemplateSSHClient   `yaml:"ssh_client,omitempty"`
	Identity    *TemplateIdentity    `yaml:"identity,omitempty"`
	TLS         *TemplateTLS         `yaml:"tls,omitempty"`
	TLSCAs      *TemplateTLSCAs      `yaml:"tls_cas,omitempty"`
	Mongo       *TemplateMongo       `yaml:"mongo,omitempty"`
	Cockroach   *TemplateCockroach   `yaml:"cockroach,omitempty"`
	Kubernetes  *TemplateKubernetes  `yaml:"kubernetes,omitempty"`
	SSHHostCert *TemplateSSHHostCert `yaml:"ssh_host_cert,omitempty"`
}

func (c *TemplateConfig) UnmarshalYAML(node *yaml.Node) error {
	// Accept either a template name (with no options) or a verbose struct, e.g.
	//   configs:
	//     - ssh_client
	//     - ssh_client:
	//         proxy_port: 1234

	var simpleTemplate string
	if err := node.Decode(&simpleTemplate); err == nil {
		switch simpleTemplate {
		case TemplateSSHClientName:
			c.SSHClient = &TemplateSSHClient{}
		case TemplateIdentityName:
			c.Identity = &TemplateIdentity{}
		case TemplateTLSName:
			c.TLS = &TemplateTLS{}
		case TemplateTLSCAsName:
			c.TLSCAs = &TemplateTLSCAs{}
		case TemplateMongoName:
			c.Mongo = &TemplateMongo{}
		case TemplateCockroachName:
			c.Cockroach = &TemplateCockroach{}
		case TemplateKubernetesName:
			c.Kubernetes = &TemplateKubernetes{}
		case TemplateSSHHostCertName:
			c.SSHHostCert = &TemplateSSHHostCert{}
		default:
			return trace.BadParameter(
				"invalid config template '%s' on line %d, expected one of: %s",
				simpleTemplate, node.Line, strings.Join(AllConfigTemplates[:], ", "),
			)
		}
		return nil
	}

	// Fall back to the full struct; alias it to get standard unmarshal
	// behavior and avoid recursion
	type rawTemplate TemplateConfig
	return trace.Wrap(node.Decode((*rawTemplate)(c)))
}

func (c *TemplateConfig) CheckAndSetDefaults() error {
	templates := []interface{ CheckAndSetDefaults() error }{
		c.SSHClient,
		c.Identity,
		c.TLS,
		c.TLSCAs,
		c.Mongo,
		c.Cockroach,
		c.Kubernetes,
		c.SSHHostCert,
	}

	notNilCount := 0
	for _, template := range templates {
		// Note: this check is fragile and will fail if the templates aren't
		// all simple pointer types. They are, though, and the "correct"
		// solution is insane, so we'll stick with this.
		if reflect.ValueOf(template).IsNil() {
			continue
		}

		if template != nil {
			if err := template.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}

			notNilCount++
		}
	}

	if notNilCount == 0 {
		return trace.BadParameter("config template must not be empty")
	} else if notNilCount > 1 {
		return trace.BadParameter("config template must have exactly one configuration")
	}

	return nil
}

// GetConfigTemplate returns the first not-nil config template implementation
// in the struct.
func (c *TemplateConfig) GetConfigTemplate() (Template, error) {
	templates := []Template{
		c.SSHClient,
		c.Identity,
		c.TLS,
		c.TLSCAs,
		c.Mongo,
		c.Cockroach,
		c.Kubernetes,
		c.SSHHostCert,
	}

	for _, template := range templates {
		// Note: same caveats as above.
		if reflect.ValueOf(template).IsNil() {
			continue
		}

		return template, nil
	}

	return nil, trace.BadParameter("no valid config template")
}

// BotConfigWriter is a trivial adapter to use the identityfile package with
// bot destinations.
type BotConfigWriter struct {
	// dest is the destination that will handle writing of files.
	dest bot.Destination

	// subpath is the subdirectory within the destination to which the files
	// should be written.
	subpath string
}

// WriteFile writes the file to the destination. Only the basename of the path
// is used. Specified permissions are ignored.
func (b *BotConfigWriter) WriteFile(name string, data []byte, _ os.FileMode) error {
	p := path.Base(name)
	if b.subpath != "" {
		p = path.Join(b.subpath, p)
	}

	return trace.Wrap(b.dest.Write(p, data))
}

// Remove removes files. This is a dummy implementation that always returns not found.
func (b *BotConfigWriter) Remove(name string) error {
	return &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// Stat checks file status. This implementation always returns not found.
func (b *BotConfigWriter) Stat(name string) (fs.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// newClientKey returns a sane client.Key for the given bot identity.
func newClientKey(ident *identity.Identity, hostCAs []types.CertAuthority) (*client.Key, error) {
	pk, err := keys.ParsePrivateKey(ident.PrivateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client.Key{
		KeyIndex: client.KeyIndex{
			ClusterName: ident.ClusterName,
		},
		PrivateKey:   pk,
		Cert:         ident.CertBytes,
		TLSCert:      ident.TLSCertBytes,
		TrustedCerts: auth.AuthoritiesToTrustedCerts(hostCAs),

		// Note: these fields are never used or persisted with identity files,
		// so we won't bother to set them. (They may need to be reconstituted
		// on tsh's end based on cert fields, though.)
		KubeTLSCerts: make(map[string][]byte),
		DBTLSCerts:   make(map[string][]byte),
	}, nil
}
