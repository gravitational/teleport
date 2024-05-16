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
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport-plugins/tooling/internal/filename"
)

// FileNames describes the location of a registry-compatible zipfile and its
// associated sidecar files
type FileNames struct {
	Zip string
	Sum string
	Sig string
}

// IsProviderTarball tests if  a given string is a Houston-compatible filename
// indicating a terraform-provider plugin type
func IsProviderTarball(fn string) bool {
	info, err := filename.Parse(fn)
	if err != nil {
		return false
	}

	return info.Type == "terraform-provider"
}

func makeFileNames(dstDir string, info filename.Info) FileNames {
	zipFileName := filepath.Join(dstDir, info.Filename(".zip"))
	return FileNames{
		Zip: zipFileName,
		Sum: zipFileName + ".sums",
		Sig: zipFileName + ".sums.sig",
	}
}

// RepackResult describes a fully-repacked provider and all of its sidecar files
type RepackResult struct {
	filename.Info
	FileNames
	Sha256        []byte
	SigningEntity *openpgp.Entity
}

// Sha256String formats the binary SHA256 as a hex string
func (r *RepackResult) Sha256String() string {
	return hex.EncodeToString(r.Sha256)
}

// RepackProvider takes a provider tarball and repacks it as a zipfile compatible
// with a terraform provider registry, generating all the required sidecar files
// as well. Returns a `RepackResult` instance containing the location of the
// generated files and information about the packed plugin
//
// For more information on the output files, see the Terraform Provider Registry
// Protocol documentation:
//
//	https://www.terraform.io/internals/provider-registry-protocol
func RepackProvider(dstDir string, srcFileName string, signingEntity *openpgp.Entity) (*RepackResult, error) {
	info, err := filename.Parse(srcFileName)
	if err != nil {
		return nil, trace.Wrap(err, "bad filename %q", srcFileName)
	}

	log.Debugf("Provider platform: %s/%s/%s", info.Version, info.OS, info.Arch)

	src, err := os.Open(srcFileName)
	if err != nil {
		return nil, trace.Wrap(err, "failed opening source file")
	}
	defer src.Close()

	// Create a temporary zipfile to repack the tarball into, which will be moved
	// into place at the end of a successful repack.
	//
	// Note that we want to create the temporary zip file in the same directory
	// where the final zip file will end up, otherwise moving the completed
	// zipfile into place may fail due to the source and destination files being
	// on different devices.
	tmpZipFile, err := os.CreateTemp(dstDir, "")
	if err != nil {
		return nil, trace.Wrap(err, "failed creating tempfile for zip archive")
	}
	defer func() {
		// we will only want to clean up the tmp file in the failure case,
		// because if RepackProvider has succeeded then the temp file has
		// already been closed and moved into place in the output directory.
		if err != nil {
			tmpZipFile.Close()
			os.Remove(tmpZipFile.Name())
		}
	}()

	log.Debugf("Repacking into zipfile: %s", tmpZipFile.Name())
	err = repack(tmpZipFile, src)
	if err != nil {
		return nil, trace.Wrap(err, "failed repacking provider")
	}

	result := &RepackResult{
		Info:          info,
		FileNames:     makeFileNames(dstDir, info),
		SigningEntity: signingEntity,
	}

	// compute sha256 and format the SHA file as per sha256sum
	_, err = tmpZipFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, trace.Wrap(err, "failed rewinding temp zipfile for summing")
	}

	var sums bytes.Buffer
	result.Sha256, err = sha256Sum(&sums, result.Zip, tmpZipFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// we're done with the temp archive for now, and we'll need the file to be
	// closed to move it into place anyway...
	tmpZipFileName := tmpZipFile.Name()
	err = tmpZipFile.Close()
	if err != nil {
		return nil, trace.Wrap(err, "failed closing temp zipfile")
	}

	// sign the sums with our private key and generate a signature
	var sig bytes.Buffer
	err = openpgp.DetachSign(&sig, signingEntity, bytes.NewReader(sums.Bytes()), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Write everything out to the dstdir
	err = writeOutput(result, tmpZipFileName, sums.Bytes(), sig.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result, nil
}

// writeOutput writes the in-memory signature data to file, and moves the temporary
// zip file into place
func writeOutput(entry *RepackResult, zipFilePath string, sums, sig []byte) error {
	log.Debugf("Writing sum file to %s", entry.Sum)
	err := ioutil.WriteFile(entry.Sum, sums, 0644)
	if err != nil {
		return trace.Wrap(err, "writing sumfile failed")
	}

	log.Debugf("Writing signature file to %s", entry.Sig)
	err = ioutil.WriteFile(entry.Sig, sig, 0644)
	if err != nil {
		return trace.Wrap(err, "writing sumfile failed")
	}

	// Do this _last_, as we want the temp file cleaned up if any of the above fails.
	log.Debugf("Moving tmp zipfile %s into place at %s", zipFilePath, entry.Zip)
	err = os.Rename(zipFilePath, entry.Zip)
	if err != nil {
		return trace.Wrap(err, "moving zipfile into place")
	}

	return nil
}
