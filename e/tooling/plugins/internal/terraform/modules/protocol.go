package modules

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// VersionsFilePath returns the file path to the versions document, which
// conforms to the Module Registry Protocol for listing versions, i.e.:
// $baseDir/:namespace/:name/:system/versions
// See: https://developer.hashicorp.com/terraform/internals/module-registry-protocol#list-available-versions-for-a-specific-module
func VersionsFilePath(baseDir, namespace, name, system string) string {
	return filepath.Join(baseDir, namespace, name, system, "versions")
}

// ModuleFilePath returns the path to a module tarball, i.e.:
// $baseDir/:namespace/:name/:system/:version.tar.gz
func ModuleFilePath(baseDir, namespace, name, system string, version semver.Version) string {
	return filepath.Join(baseDir, namespace, name, system, version.String()+".tar.gz")
}

// Versions is a Terraform Module Registry Protocol JSON index file containing
// the published versions of a Terraform module.
type Versions struct {
	// Modules array in the response always includes the requested module as the first element.
	// Terraform does not use the other elements of this list.
	// Third-party implementations should always use a single-element list for forward compatiblity.
	// https://developer.hashicorp.com/terraform/internals/module-registry-protocol#list-available-versions-for-a-specific-module
	Modules []Module `json:"modules"`
}

// Append appends new versions to the index and compacts duplicates.
func (vs *Versions) Append(newVersions ...Version) *Versions {
	if len(vs.Modules) == 0 {
		vs.Modules = append(vs.Modules, Module{})
	}
	versionsMap := map[semver.Version]Version{}
	for _, existing := range vs.Modules[0].Versions {
		versionsMap[existing.Version] = existing
	}
	for _, newVersion := range newVersions {
		versionsMap[newVersion.Version] = newVersion
	}

	semverPtrs := make([]*semver.Version, 0, len(versionsMap))
	for _, v := range versionsMap {
		semverPtrs = append(semverPtrs, &v.Version)
	}
	semver.Sort(semverPtrs)

	vs.Modules[0].Versions = make([]Version, 0, len(semverPtrs))
	for _, v := range semverPtrs {
		vs.Modules[0].Versions = append(vs.Modules[0].Versions, versionsMap[*v])
	}

	return vs
}

// Save serializes a Versions record to JSON and writes it to the given path.
func (vs *Versions) Save(filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return trace.Wrap(err, "Creating download dir")
	}

	indexFile, err := os.Create(filePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer indexFile.Close()

	encoder := json.NewEncoder(indexFile)
	return encoder.Encode(vs)
}

// LoadModulesVersionFile reads and parses a versions structure from the file
// at the supplied filesystem location
func LoadModulesVersionFile(filePath string) (*Versions, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, trace.NotFound("versions file not found")
		}
		return nil, trace.Wrap(err, "failed opening versions file")
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var m Versions
	if err = decoder.Decode(&m); err != nil {
		return nil, trace.Wrap(err, "failed decoding versions file")
	}

	return &m, nil
}

// Module contains versions for a module.
type Module struct {
	// Versions are the versions published for the module.
	Versions []Version `json:"versions"`
}

// Version contains a Terraform module version.
type Version struct {
	// Version is the published version for a module.
	Version semver.Version `json:"version"`
}
