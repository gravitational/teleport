package artifacts

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/gravitational/trace"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
)

// FindAndUpload finds all of the files referenced by the supplied patterns
// and uploads them to the supplied GCS bucket. The supplied patterns are
// expected to be fully-qualified paths, and will be searched without changing
// the current directory.
//
// Artifacts from various paths will be aggregated into one place in the
// bucket with the supplied prefix, using the file's base name to disambiguate,
// so be wary of including multiple artifacts with the same name.
//
// For example, given a directory tree that looks like
//   /
//   ├── mystery-machine.yaml
//   ├── dogs
//   │   └── scooby.yaml
//   └── people
//       └── shaggy.yaml
//
// calling
//   Upload(ctx, "bucket-name", "build-unique-id", []string{"/**/*.yaml"}))
//
// will result in a bucket with the added objects
//
// - "build-unique-id/mystery-machine.yaml"
// - "build-unique-id/scooby.yaml"
// - "build-unique-id/shaggy.yaml"
//
func FindAndUpload(ctx context.Context, bucketName, objectPrefix string, artifactPatterns []string) error {
	log.Printf("Scanning for artifacts...")
	artifacts := []string{}
	for _, pattern := range artifactPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Failed scanning for artifacts matching %q: %s", pattern, err.Error())
			continue
		}
		artifacts = append(artifacts, matches...)
	}

	if len(artifacts) == 0 {
		return nil
	}

	return Upload(ctx, bucketName, objectPrefix, artifacts...)
}

// Upload uploads a set of files to the indicated artefact bucket with the
// supplied prefix.
//
// Note that artifacts from various paths will be aggregated into one place in
// the bucket under the supplied prefix, using the file's base name to
// disambiguate. Be wary of including multiple artifacts with the same name, as
// later object may clobber earlier ones.
//
func Upload(ctx context.Context, bucket string, prefix string, files ...string) error {
	var uploadErrors *multierror.Error

	client, err := storage.NewClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	bucketHandle := client.Bucket(bucket)

	for _, filename := range files {
		objectName := path.Join(prefix, path.Base(filename))
		log.Infof("Uploading artifact %q as %q", filename, objectName)

		if err = uploadFile(ctx, bucketHandle, objectName, filename); err != nil {
			log.WithError(err).Warnf("Artifact upload failed for %q", filename)
			uploadErrors = multierror.Append(uploadErrors, err)
			continue
		}
	}

	return uploadErrors.ErrorOrNil()
}

// uploadFile uploads an individual file to the supplied storage bucket.
func uploadFile(ctx context.Context, bucket *storage.BucketHandle, objectName, filename string) error {
	obj := bucket.Object(objectName)

	source, err := os.Open(filename)
	if err != nil {
		return trace.Wrap(err, "Failed opening file to upload")
	}
	defer source.Close()

	sink := obj.NewWriter(ctx)
	_, err = io.Copy(sink, source)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note that in most cases, any upload errors won't be reported
	// until the writer is closed, so we can't just do a simple
	// `defer close()`
	if err = sink.Close(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}
