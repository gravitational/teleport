package conv

import (
	"strconv"
	"strings"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

const weakETagPrefix = "W/"

// VersionAsETag formats the supplied Teleport resource version into a HTTP ETag-
// compatible string, as per RFC-7644 (SCIM) §3.14 and RFC-9110 (HTTP Semantics)
// §8.8.3
func VersionAsETag(version string) string {
	return weakETagPrefix + strconv.Quote(version)
}

// ResourceVersion extracts the resource version string and converts it from an
// HTTP ETag-compatible string into a Teleport resource version.
func ResourceVersion(r *scimpb.Resource) string {
	if r == nil {
		return ""
	}

	etag := r.GetMeta().GetVersion()
	if etag == "" {
		return ""
	}

	// As per RFC-7644 (SCIM) §3.14 and RFC-9110 (HTTP Semantics) §8.8.3, the
	// resource version is formatted as an ETag. We will need to strip any weak
	// ETag prefix and unquote it to turn it into a Teleport resource version

	etag = strings.TrimPrefix(etag, weakETagPrefix)
	if version, err := strconv.Unquote(etag); err == nil {
		return version
	}

	// `strconv.Unquote()` treats an un-quoted string as an error, so we will
	// end up here if the client presents a legacy, un-quoted version string.
	// Using the unprocessed etag here gives a legacy client a _chance_ of being
	// right. If `strconv.Unquote()` fails for reasons *other* than `etag` being
	// unquoted, then the presented etag string will not be valid Teleport resource
	// version anyway, and will be rejected when the resource update is attempted.

	return etag
}
