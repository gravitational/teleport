package gofakes3

import "time"

const (
	// From https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingMetadata.html:
	//	"The name for a key is a sequence of Unicode characters whose UTF-8
	//	encoding is at most 1024 bytes long."
	KeySizeLimit = 1024

	// From https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingMetadata.html:
	//	Within the PUT request header, the user-defined metadata is limited to 2
	// 	KB in size. The size of user-defined metadata is measured by taking the
	// 	sum of the number of bytes in the UTF-8 encoding of each key and value.
	//
	// As this does not specify KB or KiB, KB is used in gofakes3. The reason
	// for this is if gofakes3 is used for testing, and your tests show that
	// 2KiB works, but Amazon uses 2KB...  that's a much worse time to discover
	// the disparity!
	DefaultMetadataSizeLimit = 2000

	// Like DefaultMetadataSizeLimit, the docs don't specify MB or MiB, so we
	// will accept 5MB for now. The Go client SDK rejects 5MB with the error
	// "part size must be at least 5242880 bytes", which is a hint that it
	// has been interpreted as MiB at least _somewhere_, but we should remain
	// liberal in what we accept in the face of ambiguity.
	DefaultUploadPartSize = 5 * 1000 * 1000

	DefaultSkewLimit = 15 * time.Minute

	MaxUploadsLimit       = 1000
	DefaultMaxUploads     = 1000
	MaxUploadPartsLimit   = 1000
	DefaultMaxUploadParts = 1000

	MaxBucketKeys        = 1000
	DefaultMaxBucketKeys = 1000

	MaxBucketVersionKeys        = 1000
	DefaultMaxBucketVersionKeys = 1000

	// From the docs: "Part numbers can be any number from 1 to 10,000, inclusive."
	MaxUploadPartNumber = 10000
)
