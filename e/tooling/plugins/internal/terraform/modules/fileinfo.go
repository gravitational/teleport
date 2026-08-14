package modules

import (
	"path/filepath"
	"regexp"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

var (
	fileNamePattern *regexp.Regexp = regexp.MustCompile(`^terraform-module_(?P<namespace>.+)_(?P<name>.+)_(?P<system>.+)_v(?P<version>.+)[.]tar[.]gz$`)
)

// ParseFilePath parses a Terraform module tarball file path of the form
// terraform-module_<namespace>_<name>_<system>_v<version>.tar.gz as [FileInfo].
func ParseFilePath(filePath string) (*FileInfo, error) {
	fileName := filepath.Base(filePath)
	matches := fileNamePattern.FindStringSubmatch(fileName)
	fileNamePattern.SubexpNames()
	if len(matches) != 5 {
		return nil, trace.Errorf("filename %q does not match required pattern, expected 5 matches, got %d", fileName, len(matches))
	}

	version := matches[fileNamePattern.SubexpIndex("version")]
	parsedVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, trace.Wrap(err, "failed parsing version %v as semver", version)
	}

	return &FileInfo{
		Path:      filePath,
		Namespace: matches[fileNamePattern.SubexpIndex("namespace")],
		Name:      matches[fileNamePattern.SubexpIndex("name")],
		System:    matches[fileNamePattern.SubexpIndex("system")],
		Version:   *parsedVersion,
	}, nil
}

// FileInfo contains parsed info about a Terraform module tarball file.
type FileInfo struct {
	// Path is the path to the module tarball.
	Path string
	// Namespace is the Terraform module registry namespace, which is typically the publishing organization's name, e.g., "teleport".
	Namespace string
	// Name is the Terraform module registry name component, e.g., "discovery".
	Name string
	// System is the Terraform module registry system component, which is the name of the remote target system, e.g., "aws".
	System string
	// Version holds the parsed module version.
	Version semver.Version
}

// IsModuleTarball returns true if the file path is a Terraform module tarball.
func IsModuleTarball(filePath string) bool {
	_, err := ParseFilePath(filePath)
	return err == nil
}
