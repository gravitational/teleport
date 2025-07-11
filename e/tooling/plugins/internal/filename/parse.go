package filename

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

var (
	filenamePattern *regexp.Regexp = regexp.MustCompile(`^(?P<plugin>.*)-teleport(?P<variant>[[:alpha:]]*)-v(?P<version>.*)-(?P<os>linux|darwin|windows)-(?P<arch>amd64|arm|arm64)-bin.tar.gz$`)
)

// Info holds information about a plugin, deduced from its Houston-compatible
// filename.
type Info struct {
	// Type represents the plugin type, e.g. "terraform-provider"
	Type string
	// Version holds the parsed plugin version number
	Version semver.Version
	// OS is the operating system the plugin was built for
	OS string
	// Arch is the CPU architecture the plugin was built for
	Arch string
	// Variant of the Terraform provider - empty for the standard Teleport
	// provider, "mwi" for the MWI provider.
	Variant string
}

// Parse attempts to deduce information about a staged plugin from its (assumed
// Houston-compatible) filename, returning an error if the filename can't be
// parsed.
func Parse(filename string) (Info, error) {
	filename = filepath.Base(filename)

	matches := filenamePattern.FindStringSubmatch(filename)
	switch len(matches) {
	case 0:
		return Info{}, trace.Errorf("filename %q does not match required pattern", filename)
	case 6:
		break
	default:
		return Info{}, trace.Errorf("filename %q does not match required pattern, expected 6 matches, got %d", filename, len(matches))
	}

	version, err := semver.NewVersion(matches[3])
	if err != nil {
		return Info{}, trace.Wrap(err, "failed parsing version as semver")
	}

	return Info{
		Type:    matches[1],
		Version: *version,
		Variant: matches[2],
		OS:      matches[4],
		Arch:    matches[5],
	}, nil
}

// Filename generates a Houston-compatible filename for the Info block, with a
// given file extension (NB: the extension is expected to include the leading
// dot).
func (info *Info) Filename(extension string) string {
	return fmt.Sprintf("%s-teleport%s-v%s-%s-%s-bin%s", info.Type, info.Variant, info.Version, info.OS, info.Arch, extension)
}
