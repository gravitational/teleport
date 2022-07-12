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

package artifacts

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gravitational/trace"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
)

// ValidatePatterns checks that the supplied patterns are rooted inside
// the given workspace. Workspace is expected to be the fully-qualified
// path to the workspace root. Returns a collection of patterns that
// have been validated and canonicalised.
func ValidatePatterns(workspace string, patterns []string) ([]string, error) {
	result := make([]string, len(patterns))
	for i, p := range patterns {
		if path.IsAbs(p) {
			return nil, trace.BadParameter("Cannot use absolute path to artifact: %s", p)
		}

		fullyQualifiedPattern, err := filepath.Abs(filepath.Join(workspace, p))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !strings.HasPrefix(fullyQualifiedPattern, workspace) {
			return nil, trace.BadParameter("artifact patten points outside of workspace: %s", p)
		}

		result[i] = fullyQualifiedPattern
	}

	return result, nil
}

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
	artifacts := find(artifactPatterns)

	if len(artifacts) == 0 {
		return nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	bucketHandle := bucketAdaptor{client.Bucket(bucketName)}

	return upload(ctx, bucketHandle, objectPrefix, artifacts)
}

// bucket presents a minimal interface to a storage bucket, allowing us to
// mock out GCB for testing
type objectHandle interface {
	NewWriter(context.Context) io.WriteCloser
}

// bucket presents an interface to a storage bucket, allowing us to
// mock out GCB for testing
type bucketHandle interface {
	Object(name string) objectHandle
}

// find searches through the supplied glob patterns and collects the matching
// file paths.
func find(artifactPatterns []string) []string {
	log.Printf("Scanning for artifacts...")
	artifacts := []string{}
	for _, pattern := range artifactPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Failed scanning for artifacts matching %q: %s", pattern, err.Error())
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				log.Warnf("Failed stating %q: %s", pattern, err.Error())
				continue
			}

			if info.IsDir() {
				continue
			}

			artifacts = append(artifacts, match)
		}
	}
	return artifacts
}

// upload uploads a set of files to the indicated artifact bucket with the
// supplied prefix.
//
// Note that artifacts from various paths will be aggregated into one place in
// the bucket under the supplied prefix, using the file's base name to
// disambiguate. Be wary of including multiple artifacts with the same name, as
// later object may clobber earlier ones.
//
func upload(ctx context.Context, bucket bucketHandle, prefix string, files []string) error {
	var uploadErrors *multierror.Error

	for _, filename := range files {
		objectName := path.Join(prefix, path.Base(filename))
		log.Infof("Uploading artifact %q as %q", filename, objectName)

		if err := uploadFile(ctx, bucket, objectName, filename); err != nil {
			log.WithError(err).Warnf("Artifact upload failed for %q", filename)
			uploadErrors = multierror.Append(uploadErrors, err)
			continue
		}
	}

	return uploadErrors.ErrorOrNil()
}

// uploadFile uploads an individual file to the supplied storage bucket.
func uploadFile(ctx context.Context, bucket bucketHandle, objectName, filename string) error {
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
