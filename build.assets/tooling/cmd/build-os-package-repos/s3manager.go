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

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gravitational/trace"
	"github.com/inhies/go-bytesize"
	"github.com/seqsense/s3sync"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type S3manager struct {
	syncManager        *s3sync.Manager
	uploader           *s3manager.Uploader
	downloader         *s3manager.Downloader
	bucketLocalPath    string
	bucketName         string
	bucketURL          *url.URL
	maxConcurrentSyncs int
	downloadedBytes    int64
}

func NewS3Manager(config *S3Config) (*S3manager, error) {
	// Right now the AWS session is only used by this manager, but if it ends
	// up being needed elsewhere then it should probably be moved to an arg
	awsSession, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new AWS session")
	}

	syncManagerMaxConcurrentSyncs := config.maxConcurrentSyncs
	if syncManagerMaxConcurrentSyncs < 0 {
		// This isn't unlimited but due to the s3sync library's parallelism implementation
		//  this must be limited to a "reasonable" number
		syncManagerMaxConcurrentSyncs = 128
	}

	s := &S3manager{
		bucketName: config.bucketName,
		bucketURL: &url.URL{
			Scheme: "s3",
			Host:   config.bucketName,
		},
		syncManager:        s3sync.New(awsSession, s3sync.WithParallel(syncManagerMaxConcurrentSyncs)),
		uploader:           s3manager.NewUploader(awsSession),
		downloader:         s3manager.NewDownloader(awsSession),
		maxConcurrentSyncs: config.maxConcurrentSyncs,
	}
	s.ChangeLocalBucketPath(config.localBucketPath)

	s3sync.SetLogger(&s3logger{})

	return s, nil
}

func (s *S3manager) ChangeLocalBucketPath(newBucketPath string) error {
	s.bucketLocalPath = newBucketPath

	// Ensure the local bucket path exists as it will be needed by all functions
	err := os.MkdirAll(s.bucketLocalPath, 0660)
	if err != nil {
		return trace.Wrap(err, "failed to ensure path %q exists", s.bucketLocalPath)
	}

	return nil
}

func (s *S3manager) DownloadExistingRepo() error {
	err := deleteAllFilesInDirectory(s.bucketLocalPath)
	if err != nil {
		return trace.Wrap(err, "failed to remove all filesystem entries in %q", s.bucketLocalPath)
	}

	downloadGroup := &errgroup.Group{}
	downloadGroup.SetLimit(s.maxConcurrentSyncs)
	linkMap := make(map[string]string)

	var continuationToken *string
	for {
		listObjResponse, err := s.downloader.S3.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:            &s.bucketName,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return trace.Wrap(err, "failed to list objects for bucket %q", s.bucketName)
		}

		for _, s3object := range listObjResponse.Contents {
			s.processS3ObjectDownload(s3object, downloadGroup, &linkMap)
		}

		continuationToken = listObjResponse.NextContinuationToken
		if continuationToken == nil {
			break
		}
	}

	// Even if an error has occurred we should wait to exit until all running syncs have
	// completed, even if not successful
	logrus.Info("Waiting for download to complete...")
	err = downloadGroup.Wait()
	if err != nil {
		return trace.Wrap(err, "failed to perform S3 sync from remote bucket %q to local bucket %q", s.bucketName, s.bucketLocalPath)
	}

	// Links must be created after their target exists
	err = createLinks(linkMap)
	if err != nil {
		return trace.Wrap(err, "failed to create filesystem links for bucket %q", s.bucketName)
	}

	logrus.Infof("Downloaded %s bytes", bytesize.New(float64(s.downloadedBytes)))
	return nil
}

func (s *S3manager) processS3ObjectDownload(s3object *s3.Object, downloadGroup *errgroup.Group, linkMap *map[string]string) {
	downloadGroup.Go(func() error {
		objectLink, err := s.getObjectLink(s3object)
		if err != nil {
			return trace.Wrap(err, "failed to get object link for key %q in bucket %q", *s3object.Key, s.bucketName)
		}

		// If the link does not start with a '/' then it is not a filesystem link
		if objectLink != nil && len(*objectLink) > 0 && (*objectLink)[0] == '/' {
			localObjectPath := filepath.Join(s.bucketLocalPath, *s3object.Key)
			linkTarget := filepath.Join(s.bucketLocalPath, *objectLink)
			(*linkMap)[localObjectPath] = linkTarget
			return nil
		}

		err = s.downloadFile(s3object)
		if err != nil {
			return trace.Wrap(err, "failed to download S3 file %q from bucket %q", *s3object.Key, s.bucketName)
		}

		return nil
	})
}

func createLinks(linkMap map[string]string) error {
	for file, target := range linkMap {
		logrus.Infof("Creating a symlink from %q to %q", file, target)
		err := os.MkdirAll(filepath.Dir(file), 0660)
		if err != nil {
			return trace.Wrap(err, "failed to create directory structure for %q", file)
		}

		err = os.Symlink(target, file)
		if err != nil {
			return trace.Wrap(err, "failed to symlink %q to %q", file, target)
		}
	}

	return nil
}

// This could potentially be made more efficient by running `os.RemoveAll` in a goroutine
//  as random access on storage devices performs better at a higher queue depth
func deleteAllFilesInDirectory(dir string) error {
	// Note that os.ReadDir does not follow/eval links which is important here
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return trace.Wrap(err, "failed to list directory entries for directory %q", dir)
	}

	for _, dirEntry := range dirEntries {
		dirEntryPath := filepath.Join(dir, dirEntry.Name())
		err = os.RemoveAll(dirEntryPath)
		if err != nil {
			return trace.Wrap(err, "failed to remove directory entry %q", dirEntryPath)
		}
	}

	return nil
}

func (s *S3manager) getObjectLink(s3object *s3.Object) (*string, error) {
	s3HeadObjectOutput, err := s.downloader.S3.HeadObject(&s3.HeadObjectInput{
		Bucket: &s.bucketName,
		Key:    s3object.Key,
		// Probably unnecessary but this will cause an error to be thrown if somebody is
		// modifying the object while this program is running
		IfMatch:           s3object.ETag,
		IfUnmodifiedSince: s3object.LastModified,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve metadata for key %q in bucket %q", *s3object.Key, s.bucketName)
	}

	return s3HeadObjectOutput.WebsiteRedirectLocation, nil
}

// s3sync has a bug when downloading a single file so this call reimplements s3sync's download
func (s *S3manager) downloadFile(s3object *s3.Object) error {
	logrus.Infof("Downloading %q...", *s3object.Key)
	localObjectPath := filepath.Join(s.bucketLocalPath, *s3object.Key)

	err := os.MkdirAll(filepath.Dir(localObjectPath), 0660)
	if err != nil {
		return trace.Wrap(err, "failed to create directory structure for %q", localObjectPath)
	}

	fileWriter, err := os.Create(localObjectPath)
	if err != nil {
		return trace.Wrap(err, "failed to open %q for writing", localObjectPath)
	}
	defer fileWriter.Close()

	fileDownloadByteCount, err := s.downloader.Download(fileWriter, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(*s3object.Key),
	})
	if err != nil {
		return trace.Wrap(err, "failed to download object %q from bucket %q to local path %q", *s3object.Key, s.bucketName, localObjectPath)
	}

	s.downloadedBytes += fileDownloadByteCount

	err = os.Chtimes(localObjectPath, *s3object.LastModified, *s3object.LastModified)
	if err != nil {
		return trace.Wrap(err, "failed to update the access and modification time on file %q to %v", localObjectPath, *s3object.LastModified)
	}

	logrus.Infof("Download %q complete", *s3object.Key)
	return nil
}

func (s *S3manager) UploadBuiltRepo() error {
	err := s.sync(false)
	if err != nil {
		return trace.Wrap(err, "failed to upload bucket")
	}

	return nil
}

func (s *S3manager) UploadBuiltRepoWithRedirects(extensionToMatch, relativeRedirectDir string) error {
	uploadGroup := &errgroup.Group{}
	uploadGroup.SetLimit(s.maxConcurrentSyncs)

	walkErr := filepath.WalkDir(s.bucketLocalPath, func(absPath string, info fs.DirEntry, err error) error {
		logrus.Debugf("Starting on %q...", absPath)

		if err != nil {
			return trace.Wrap(err, "failed to walk over directory %q on path %q", s.bucketLocalPath)
		}

		syncFunc, err := s.syncGenericFsObject(absPath, info)
		if err != nil {
			return trace.Wrap(err, "failed to get syncing function for %q", absPath)
		}

		uploadGroup.Go(syncFunc)
		logrus.Debugf("Upload for %q queued", absPath)
		return nil
	})

	// Even if an error has occurred we should wait to exit until all running syncs have
	// completed, even if not successful
	logrus.Info("Waiting for sync to complete...")
	syncErr := uploadGroup.Wait()
	// Future work: add upload logging information once
	// https://github.com/seqsense/s3sync/commit/29b3fcb259293d80634cb3916e0f28467d017087 has been released
	logrus.Info("Sync has completed")

	errs := make([]error, 0, 2)
	if walkErr != nil {
		errs = append(errs, trace.Wrap(walkErr, "failed to walk over entries in %q", s.bucketLocalPath))
	}

	if syncErr != nil {
		errs = append(errs, trace.Wrap(syncErr, "failed to perform S3 sync from local bucket %q to remote bucket %q", s.bucketLocalPath, s.bucketName))
	}

	if len(errs) > 0 {
		return trace.Wrap(trace.NewAggregate(errs...), "one or more erros occurred while uploading built repo %q", s.bucketLocalPath)
	}

	return nil
}

func (s *S3manager) syncGenericFsObject(absPath string, dirEntryInfo fs.DirEntry) (func() error, error) {
	// Don't do anything with non-empty directories as they will be caught later by their contents
	if dirEntryInfo.IsDir() {
		f, err := s.buildSyncDirFunc(absPath)
		if err != nil {
			return nil, trace.Wrap(err, "failed to build directory syncing function to sync %q", absPath)
		}

		return f, nil
	} else
	// If symbolic link
	if dirEntryInfo.Type()&fs.ModeSymlink != 0 {
		f, err := s.buildSyncSymbolicLinkFunc(absPath)
		if err != nil {
			return nil, trace.Wrap(err, "failed to build symbolic link file syncing function to sync %q", absPath)
		}

		return f, nil
	}

	// sync a single file or directory
	f, err := s.buildSyncSingleFsEntryFunc(absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build single file syncing function to sync %q", absPath)
	}

	return f, nil
}

func (s *S3manager) buildSyncDirFunc(absPath string) (func() error, error) {
	isDirEmpty, err := isDirectoryEmpty(absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine if directory %q is empty", absPath)
	}

	if !isDirEmpty {
		logrus.Debug("Skipping non-empty directory")
		return func() error { return nil }, nil
	}

	// If the directory has no contents, call sync normally which will create the directory remotely if not exists
	f, err := s.buildSyncSingleFsEntryFunc(absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build single file syncing function to sync %q", absPath)
	}

	return f, nil
}

func (s *S3manager) buildSyncSymbolicLinkFunc(absPath string) (func() error, error) {
	actualFilePath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to follow symlink for path %q", absPath)
	}

	isInBucket, err := isPathChildOfAnother(s.bucketLocalPath, actualFilePath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine if %q is a child of %q", actualFilePath, s.bucketLocalPath)
	}

	if isInBucket {
		// This will re-upload every redirect file ever created. Implementing "sync" functionality would
		// require significantly more engineering effort and this cost is low so this shouldn't be a
		// problem.
		return func() error {
			err := s.UploadRedirectFile(absPath, actualFilePath)
			if err != nil {
				return trace.Wrap(err, "failed to upload a redirect file to S3 for %q targeting %q", absPath, actualFilePath)
			}

			return nil
		}, nil
	}

	// If not in bucket, call sync normally which will follow the symlink to the actual file and upload it
	f, err := s.buildSyncSingleFsEntryFunc(absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build single file syncing function to sync %q", absPath)
	}

	return f, nil
}

func (s *S3manager) buildSyncSingleFsEntryFunc(absPath string) (func() error, error) {
	relPath, err := filepath.Rel(s.bucketLocalPath, absPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get %q relative to %q", absPath, s.bucketLocalPath)
	}

	remoteURL := getURLWithPath(*s.bucketURL, relPath)
	return func() error {
		err := s.syncManager.Sync(absPath, remoteURL)
		if err != nil {
			return trace.Wrap(err, "failed to sync from %q to %q", absPath, remoteURL)
		}

		return nil
	}, nil
}

func getURLWithPath(baseURL url.URL, path string) string {
	// Because this function is pass-by-value it should not modify `baseUrl`, where doing this directly on the
	// provided parameter would modify it
	baseURL.Path = path
	return baseURL.String()
}

func isPathChildOfAnother(baseAbsPath string, testAbsPath string) (bool, error) {
	// General implementation from https://stackoverflow.com/questions/28024731/check-if-given-path-is-a-subdirectory-of-another-in-golang
	relPath, err := filepath.Rel(baseAbsPath, testAbsPath)
	if err != nil {
		return false, trace.Wrap(err, "failed to get the path of %q relative to %q", testAbsPath, baseAbsPath)
	}

	return !strings.HasPrefix(relPath, fmt.Sprintf("..%c", os.PathSeparator)) && relPath != "..", nil
}

func (s *S3manager) UploadRedirectFile(localAbsSrcPath, localAbsRemoteTargetPath string) error {
	relSrcPath, err := filepath.Rel(s.bucketLocalPath, localAbsSrcPath)
	if err != nil {
		return trace.Wrap(err, "failed to get %q relative to %q", localAbsSrcPath, s.bucketLocalPath)
	}

	relTargetPath, err := filepath.Rel(s.bucketLocalPath, localAbsRemoteTargetPath)
	if err != nil {
		return trace.Wrap(err, "failed to get %q relative to %q", localAbsRemoteTargetPath, s.bucketLocalPath)
	}

	logrus.Infof("Creating a redirect file from %q to %q", relSrcPath, relTargetPath)
	// S3 requires a prepended "/" to inform the redirect metadata that the target is another S3 object
	// in the same bucket
	s3TargetPath := filepath.Join("/", relTargetPath)
	// Upload an empty file that when requested will redirect to the real one
	_, err = s.uploader.Upload(&s3manager.UploadInput{
		Bucket:                  &s.bucketName,
		Key:                     &relSrcPath,
		Body:                    bytes.NewReader([]byte{}),
		WebsiteRedirectLocation: &s3TargetPath,
	})
	if err != nil {
		return trace.Wrap(err, "failed to upload an empty redirect file to %q in bucket %q", relSrcPath, s.bucketName)
	}

	return nil
}

func (s *S3manager) UploadRedirectURL(remoteAbsSourcePath, targetURL string) error {
	logrus.Infof("Creating redirect from %q to %q", remoteAbsSourcePath, targetURL)

	_, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket:                  &s.bucketName,
		Key:                     &remoteAbsSourcePath,
		Body:                    bytes.NewReader([]byte{}),
		WebsiteRedirectLocation: &targetURL,
	})

	if err != nil {
		return trace.Wrap(err, "failed to upload URL redirect file targeting %q to %q", targetURL, remoteAbsSourcePath)
	}

	return nil
}

func isDirectoryEmpty(dirPath string) (bool, error) {
	// Pulled from https://stackoverflow.com/questions/30697324/how-to-check-if-directory-on-path-is-empty
	f, err := os.Open(dirPath)
	if err != nil {
		return false, trace.Wrap(err, "failed to open directory %q", dirPath)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	if err != nil {
		return false, trace.Wrap(err, "failed to read the name of directories in %q", dirPath)
	}

	return false, nil
}

func (s *S3manager) sync(download bool) error {
	var src, dest string
	if download {
		src = s.bucketURL.String()
		dest = s.bucketLocalPath
	} else {
		src = s.bucketLocalPath
		dest = s.bucketURL.String()
	}

	logrus.Infof("Performing S3 sync from %q to %q...", src, dest)
	err := s.syncManager.Sync(src, dest)
	if err != nil {
		return trace.Wrap(err, "failed to sync %q to %q", src, dest)
	}
	logrus.Infoln("S3 sync complete")

	return nil
}
