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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/tool/tbot/destination"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
)

// BotConfigWriter is a trivial adapter to use the identityfile package with
// bot destinations.
type BotConfigWriter struct {
	// dest is the destination that will handle writing of files.
	dest destination.Destination

	// subpath is the subdirectory within the destination to which the files
	// should be written.
	subpath string
}

// WriteFile writes the file to the destination. Only the basename of the path
// is used. Specified permissions are ignored.
func (b *BotConfigWriter) WriteFile(name string, data []byte, perm os.FileMode) error {
	p := path.Join(b.subpath, path.Base(name))
	log.Debugf("WriteFile(%q, ...) to %q", name, p)
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

// isFormatValid checks if the given format is a supported file format.s
func isFormatValid(format string) bool {
	for _, f := range identityfile.KnownFileFormats {
		if string(f) == format {
			return true
		}
	}

	return false
}

type TemplateIdentityFile struct {
	Formats []string `yaml:"formats,omitempty"`
}

func (t *TemplateIdentityFile) CheckAndSetDefaults() error {
	for _, format := range t.Formats {
		if !isFormatValid(format) {
			return trace.BadParameter("invalid format %q, expected one of: %s", format, identityfile.KnownFileFormats.String())
		}
	}

	return nil
}

func (t *TemplateIdentityFile) Describe() []FileDescription {
	var descriptions []FileDescription

	// identityfile may generate multiple files
	for _, format := range t.Formats {
		descriptions = append(descriptions, FileDescription{
			Name:  format,
			IsDir: true,
		})
	}

	return descriptions
}

func (t *TemplateIdentityFile) Render(ctx context.Context, authClient auth.ClientI, currentIdentity *identity.Identity, destination *DestinationConfig) error {
	if !destination.ContainsKind(identity.KindSSH) {
		return trace.BadParameter("%s config template requires kind `ssh` to be enabled", TemplateIdentityFileName)
	}

	if !destination.ContainsKind(identity.KindTLS) {
		return trace.BadParameter("%s config template requires kind `tls` to be enabled", TemplateIdentityFileName)
	}

	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	hostCAs, err := authClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, format := range t.Formats {
		cfg := identityfile.WriteConfig{
			// Hard code all written files as "identity" for now. We could make
			// this user configurable if desired.
			OutputPath: "identity",
			Writer: &BotConfigWriter{
				dest:    dest,
				subpath: format,
			},
			Key: &client.Key{
				KeyIndex: client.KeyIndex{
					ClusterName: currentIdentity.ClusterName,
				},
				Priv:      currentIdentity.PrivateKeyBytes,
				Pub:       currentIdentity.PublicKeyBytes,
				Cert:      currentIdentity.CertBytes,
				TLSCert:   currentIdentity.TLSCertBytes,
				TrustedCA: auth.AuthoritiesToTrustedCerts(hostCAs),

				// TODO: configure these? we have a 1:1 mapping of destination
				// -> app/db/kube cert so this should be knowable.
				KubeTLSCerts: make(map[string][]byte),
				DBTLSCerts:   make(map[string][]byte),
			},
			Format: identityfile.Format(format),

			// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
			OverwriteDestination: true,

			// TODO: KubeProxyAddr: ?,
		}

		files, err := identityfile.Write(cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Debugf("Wrote identityfile entries: %+v", files)
	}

	return nil
}
