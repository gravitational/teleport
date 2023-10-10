package externalcloudaudit

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

func ParseS3URI(bucket string) (host string, prefix string, err error) {
	u, err := url.Parse(bucket)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if strings.ToLower(u.Scheme) != "s3" {
		return "", "", trace.BadParameter("bucket: %s must start with s3://", bucket)
	}

	return u.Host, u.Path, nil
}

func ValidateBuckets(buckets ...string) error {
	bucketMap := map[string]map[string]struct{}{}

	for _, bucket := range buckets {
		host, prefix, err := ParseS3URI(bucket)
		if err != nil {
			return trace.Wrap(err)
		}

		if prefix == "" {
			prefix = "/"
		}

		if b, ok := bucketMap[host]; ok {
			// If bucket already exists check existing prefixes for invalid configuration
			for p := range b {
				if p == prefix {
					return trace.BadParameter("At least two s3 buckets share the exact bucket and prefix path")
				}

				if strings.HasPrefix(p, prefix) || strings.HasPrefix(prefix, p) {
					return trace.BadParameter("A prefix is not allowed to be a sub prefix of another")
				}
			}
		} else {
			// Add host and prefix to map
			bucketMap[host] = map[string]struct{}{prefix: {}}
		}

	}

	return nil
}
