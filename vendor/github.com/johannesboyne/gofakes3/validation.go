package gofakes3

import (
	"net"
	"regexp"
	"strings"
)

// This pattern can be used to match both the entire bucket name (including period-
// separated labels) and the individual label components, presuming you have already
// split the string by period.
var bucketNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9\.-]+)[a-z0-9]$`)

// ValidateBucketName applies the rules from the AWS docs:
// https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html#bucketnamingrules
//
// 1. Bucket names must comply with DNS naming conventions.
// 2. Bucket names must be at least 3 and no more than 63 characters long.
// 3. Bucket names must not contain uppercase characters or underscores.
// 4. Bucket names must start with a lowercase letter or number.
//
// The DNS RFC confirms that the valid range of characters in an LDH label is 'a-z0-9-':
// https://tools.ietf.org/html/rfc5890#section-2.3.1
//
func ValidateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return ErrorMessage(ErrInvalidBucketName, "bucket name must be >= 3 characters and <= 63")
	}
	if !bucketNamePattern.MatchString(name) {
		return ErrorMessage(ErrInvalidBucketName, "bucket must start and end with 'a-z, 0-9', and contain only 'a-z, 0-9, -' in between")
	}

	if net.ParseIP(name) != nil {
		return ErrorMessage(ErrInvalidBucketName, "bucket names must not be formatted as an IP address")
	}

	// Bucket names must be a series of one or more labels. Adjacent labels are
	// separated by a single period (.). Bucket names can contain lowercase
	// letters, numbers, and hyphens. Each label must start and end with a
	// lowercase letter or a number.
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if !bucketNamePattern.MatchString(label) {
			return ErrorMessage(ErrInvalidBucketName, "label must start and end with 'a-z, 0-9', and contain only 'a-z, 0-9, -' in between")
		}
	}

	return nil
}

var etagPattern = regexp.MustCompile(`^"[a-z0-9]+"$`)

func validETag(v string) bool {
	return etagPattern.MatchString(v)
}
