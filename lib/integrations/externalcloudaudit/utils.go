package externalcloudaudit

import (
	"errors"
	"net/url"
	"strings"

	"github.com/aws/smithy-go"
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

// isNoSuchBucketError returns true if error is NoSuchBucket error
// Despite https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/#api-error-responses
// It doesn't seem possible to actually handle that error using s3types.NoSuchBucket.
// See https://github.com/aws/aws-sdk-go-v2/issues/1110 for discussion
func isNoSuchBucketError(err error) bool {
	var aerr smithy.APIError
	if errors.As(err, &aerr) {
		if aerr.ErrorCode() == "NoSuchBucket" {
			return true
		}
	}

	return false
}
