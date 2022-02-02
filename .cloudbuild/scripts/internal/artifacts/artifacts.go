package artifacts

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

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

func Upload(ctx context.Context, bucket string, prefix string, files ...string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	bucketHandle := client.Bucket(bucket)

	for _, filename := range files {
		objectName := path.Join("/", prefix, path.Base(filename))
		log.Infof("Uploading artifact %q as %q", filename, objectName)

		if err = uploadFile(ctx, bucketHandle, objectName, filename); err != nil {
			log.WithError(err).Warnf("Artifact upload failed for %q", filename)
			continue
		}
	}

	return nil
}

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

	if err = sink.Close(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}
