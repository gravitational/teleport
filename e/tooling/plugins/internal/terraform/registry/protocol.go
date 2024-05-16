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
package registry

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

const (
	versionsFilename = "versions"
)

func VersionsFilePath(prefix, namespace, name string) string {
	return path.Join(prefix, namespace, name, versionsFilename)
}

// Versions is a Go representation of the Terraform Registry Protocol
// `versions` file
type Versions struct {
	Versions []Version `json:"versions"`
}

// Save formats and writes the version list to the supplied filesystem
// location.
func (idx *Versions) Save(filename string) error {
	indexFile, err := os.Create(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer indexFile.Close()

	encoder := json.NewEncoder(indexFile)
	encoder.SetIndent("", "  ")

	return encoder.Encode(idx)
}

// LoadVersionsFile reads and parses a versions structure from the file
// at the supplied filesystem location
func LoadVersionsFile(filename string) (Versions, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Versions{}, trace.Wrap(err, "failed opening versions file")
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var idx Versions
	if err = decoder.Decode(&idx); err != nil {
		return Versions{}, trace.Wrap(err, "failed decoding versions file")
	}

	return idx, nil
}

// Platform encodes an OS/Arch pair
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Version describes tha compatability and available platforms for a given
// provider version
type Version struct {
	Version   semver.Version `json:"version"`
	Protocols []string       `json:"protocols"`
	Platforms []Platform     `json:"platforms"`
}

// GpgPublicKey describes a (the public half) of a GPG key used to sign a
// provider zipfile.
type GpgPublicKey struct {
	KeyID          string `json:"key_id"`
	ASCIIArmor     string `json:"ascii_armor"`
	TrustSignature string `json:"trust_signature"`
	Source         string `json:"source,omitempty"`
	SourceURL      string `json:"source_url,omitempty"`
}

// SigningKeys describes the key (or keys) that signed a given download. As
// per the registry protocol, only GPG keys are supported for now.
type SigningKeys struct {
	GpgPublicKeys []GpgPublicKey `json:"gpg_public_keys"`
}

// Download describes the file specific download package for a given platform &
// provider version. If a provider supports multiple OSs & architectures, the
// registry will contain multiple Download records, one for each unique
// architecture/OS pair.
type Download struct {
	Protocols    []string    `json:"protocols"`
	OS           string      `json:"os"`
	Arch         string      `json:"arch"`
	Filename     string      `json:"filename"`
	DownloadURL  string      `json:"download_url"`
	ShaURL       string      `json:"shasums_url"`
	SignatureURL string      `json:"shasums_signature_url"`
	Sha          string      `json:"shasum"`
	SigningKeys  SigningKeys `json:"signing_keys"`
}

// Save serializes a Download record to JSON and writes it to the appropriate
// Provider Registry Protocol-defined location in a registry directory tree, i.e.
//
//	indexDir/:namespace/:name/:version/download/:os/:arch
func (dl *Download) Save(indexDir, namespace, name string, version semver.Version) (string, error) {
	filename := filepath.Join(indexDir, namespace, name, version.String(), "download", dl.OS, dl.Arch)

	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return "", trace.Wrap(err, "Creating download dir")
	}

	f, err := os.Create(filename)
	if err != nil {
		return "", trace.Wrap(err, "Failed opening file")
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")

	return filename, encoder.Encode(dl)
}

// NewDownloadFromRepackResult creates a new, fully-populated Download object
// from the result of repacking a Houston-compatible tarball into a Terraform
// registry-compatible zip
// archive
func NewDownloadFromRepackResult(info *RepackResult, protocols []string, objectStoreURL string) (*Download, error) {

	keyText, err := formatPublicKey(info.SigningEntity)
	if err != nil {
		return nil, trace.Wrap(err, "failed formatting public key")
	}

	return &Download{
		Protocols:    protocols,
		OS:           info.OS,
		Arch:         info.Arch,
		Filename:     filepath.Base(info.Zip),
		Sha:          info.Sha256String(),
		DownloadURL:  objectStoreURL + filepath.Base(info.Zip),
		ShaURL:       objectStoreURL + filepath.Base(info.Sum),
		SignatureURL: objectStoreURL + filepath.Base(info.Sig),
		SigningKeys: SigningKeys{
			GpgPublicKeys: []GpgPublicKey{
				{
					KeyID:      info.SigningEntity.PrivateKey.PublicKey.KeyIdString(),
					ASCIIArmor: keyText,
				},
			},
		},
	}, nil
}

func formatPublicKey(signingKey *openpgp.Entity) (string, error) {
	var text bytes.Buffer
	writer, err := armor.Encode(&text, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	err = signingKey.Serialize(writer)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err = writer.Close(); err != nil {
		return "", trace.Wrap(err)
	}

	return text.String(), nil
}
