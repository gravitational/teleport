package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"
	"github.com/seqsense/s3sync"
	"github.com/sirupsen/logrus"
)

type s3manager struct {
	syncManager *s3sync.Manager
	bucketName  string
	bucketPath  string
}

func NewS3Manager(bucketName string) *s3manager {
	// Right now the AWS session is only used by this manager, but if it ends
	// up being needed elsewhere then it should probably be moved to an arg
	awsSession := session.Must(session.NewSession())

	manager := &s3manager{
		syncManager: s3sync.New(awsSession),
		bucketName:  bucketName,
		bucketPath:  fmt.Sprintf("s3://%s", bucketName),
	}

	s3sync.SetLogger(&s3logger{})

	return manager
}

func (s *s3manager) DownloadExistingRepo(localPath string) error {
	err := ensureDirectoryExists(localPath)
	if err != nil {
		return trace.Wrap(err, "failed to ensure path %q exists", localPath)
	}

	err = s.sync(localPath, true)
	if err != nil {
		return trace.Wrap(err, "failed to download bucket")
	}

	return nil
}

func (s *s3manager) UploadBuiltRepo(localPath string) error {
	err := s.sync(localPath, false)

	if err != nil {
		return trace.Wrap(err, "failed to upload bucket")
	}

	return nil
}

func (s *s3manager) sync(localPath string, download bool) error {
	var src, dest string
	if download {
		src = s.bucketPath
		dest = localPath
	} else {
		src = localPath
		dest = s.bucketPath
	}

	logrus.Infof("Performing S3 sync from %q to %q...", src, dest)
	err := s.syncManager.Sync(src, dest)
	if err != nil {
		return trace.Wrap(err, "failed to sync %q to %q", src, dest)
	}
	logrus.Infoln("S3 sync complete")

	return nil
}

func ensureDirectoryExists(path string) error {
	err := os.MkdirAll(path, 0660)
	if err != nil {
		return trace.Wrap(err, "failed to create directory %q", path)
	}

	return nil
}
