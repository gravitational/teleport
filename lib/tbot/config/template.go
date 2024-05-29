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

package config

import (
	"context"
	"io/fs"
	"os"
	"path"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
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

// FileDescription is a minimal spec needed to create an empty end-user-owned
// file with bot-writable ACLs during `tbot init`.
type FileDescription struct {
	// Name is the name of the file or directory to create.
	Name string

	// IsDir designates whether this describes a subdirectory inside the
	// Destination.
	IsDir bool
}

// template defines functions for dynamically writing additional files to
// a Destination.
type template interface {
	// Name returns the name of this config template.
	name() string

	// Describe generates a list of all files this ConfigTemplate will generate
	// at runtime. Currently ConfigTemplates are required to know this
	// statically as this must be callable without any auth clients (or any
	// secrets) for use with `tbot init`. If an arbitrary number of files must
	// be generated, they should be placed in a subdirectory.
	describe() []FileDescription

	// Render writes the config template to the Destination.
	render(
		ctx context.Context,
		p provider,
		identity *identity.Identity,
		destination bot.Destination,
	) error
}

// BotConfigWriter is a trivial adapter to use the identityfile package with
// bot destinations.
type BotConfigWriter struct {
	ctx context.Context

	// dest is the Destination that will handle writing of files.
	dest bot.Destination

	// subpath is the subdirectory within the Destination to which the files
	// should be written.
	subpath string
}

// WriteFile writes the file to the Destination. Only the basename of the path
// is used. Specified permissions are ignored.
func (b *BotConfigWriter) WriteFile(name string, data []byte, _ os.FileMode) error {
	p := path.Base(name)
	if b.subpath != "" {
		p = path.Join(b.subpath, p)
	}

	return trace.Wrap(b.dest.Write(b.ctx, p, data))
}

// Remove removes files. This is a dummy implementation that always returns not found.
func (b *BotConfigWriter) Remove(name string) error {
	return &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// Stat checks file status. This implementation always returns not found.
func (b *BotConfigWriter) Stat(name string) (fs.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// ReadFile reads a given file. This implementation always returns not found.
func (b *BotConfigWriter) ReadFile(name string) ([]byte, error) {
	return nil, &os.PathError{Op: "read", Path: name, Err: os.ErrNotExist}
}

// compile-time assertion that the BotConfigWriter implements the
// identityfile.ConfigWriter interface
var _ identityfile.ConfigWriter = (*BotConfigWriter)(nil)

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
		TrustedCerts: authclient.AuthoritiesToTrustedCerts(hostCAs),

		// Note: these fields are never used or persisted with identity files,
		// so we won't bother to set them. (They may need to be reconstituted
		// on tsh's end based on cert fields, though.)
		KubeTLSCerts: make(map[string][]byte),
		DBTLSCerts:   make(map[string][]byte),
	}, nil
}

// executablePathGetter is a function that returns the path of the current
// executable.
type executablePathGetter = func() (string, error)
