package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gravitational/trace"
	"github.com/inhies/go-bytesize"
	"github.com/seqsense/s3sync"
	"golang.org/x/sync/errgroup"
)

type S3manager struct {
	syncManager         *s3sync.Manager
	uploader            *s3manager.Uploader
	downloader          *s3manager.Downloader
	bucketLocalPath     string
	bucketName          string
	bucketURL           *url.URL
	maxConcurrentSyncs  int
	downloadedBytes     int64
	downloadedByteMutex *sync.RWMutex
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
		syncManager:         s3sync.New(awsSession, s3sync.WithParallel(syncManagerMaxConcurrentSyncs)),
		uploader:            s3manager.NewUploader(awsSession),
		downloader:          s3manager.NewDownloader(awsSession),
		maxConcurrentSyncs:  config.maxConcurrentSyncs,
		downloadedByteMutex: &sync.RWMutex{},
	}
	s.ChangeLocalBucketPath(config.localBucketPath)

	return s, nil
}

func (s *S3manager) ChangeLocalBucketPath(newBucketPath string) error {
	s.bucketLocalPath = newBucketPath

	_, err := os.Stat(s.bucketLocalPath)
	if err == nil {
		return nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err, "failed to determine if directory %q exists", s.bucketLocalPath)
	}

	slog.InfoContext(context.Background(), "Creating local bucket directory", "bucket_path", s.bucketLocalPath)
	err = os.MkdirAll(s.bucketLocalPath, 0770)
	if err != nil {
		return trace.Wrap(err, "failed to create locak bucket directory %q", s.bucketLocalPath)
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
	mapMutex := sync.RWMutex{}
	linkMap := make(map[string]string)
	mkdirMutex := sync.Mutex{}

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
			s.processS3ObjectDownload(s3object, downloadGroup, &linkMap, &mapMutex, &mkdirMutex)
		}

		continuationToken = listObjResponse.NextContinuationToken
		if continuationToken == nil {
			break
		}
	}

	// Even if an error has occurred we should wait to exit until all running syncs have
	// completed, even if not successful
	slog.InfoContext(context.Background(), "Waiting for download to complete")
	err = downloadGroup.Wait()
	if err != nil {
		return trace.Wrap(err, "failed to perform S3 sync from remote bucket %q to local bucket %q", s.bucketName, s.bucketLocalPath)
	}

	// Links must be created after their target exists
	// Mutex lock here is unnecessary as the code currently stands, but is a safety against naive future changes
	mapMutex.RLock()
	err = createLinks(linkMap)
	mapMutex.RUnlock()
	if err != nil {
		return trace.Wrap(err, "failed to create filesystem links for bucket %q", s.bucketName)
	}

	s.downloadedByteMutex.RLock()
	slog.InfoContext(context.Background(), "S3 sync completed", "total_bytes_synced", bytesize.New(float64(s.downloadedBytes)))
	s.downloadedByteMutex.RUnlock()
	return nil
}

func (s *S3manager) processS3ObjectDownload(s3object *s3.Object, downloadGroup *errgroup.Group,
	linkMap *map[string]string, mapMutex *sync.RWMutex, mkdirMutex *sync.Mutex) {
	downloadGroup.Go(func() error {
		objectLink, err := s.getObjectLink(s3object)
		if err != nil {
			return trace.Wrap(err, "failed to get object link for key %q in bucket %q", *s3object.Key, s.bucketName)
		}

		// If the link does not start with a '/' then it is not a filesystem link
		if objectLink != nil && len(*objectLink) > 0 && (*objectLink)[0] == '/' {
			localObjectPath := filepath.Join(s.bucketLocalPath, *s3object.Key)
			linkTarget := filepath.Join(s.bucketLocalPath, *objectLink)
			mapMutex.Lock()
			(*linkMap)[localObjectPath] = linkTarget
			mapMutex.Unlock()
			return nil
		}

		err = s.downloadFile(s3object, mkdirMutex)
		if err != nil {
			return trace.Wrap(err, "failed to download S3 file %q from bucket %q", *s3object.Key, s.bucketName)
		}

		return nil
	})
}

func createLinks(linkMap map[string]string) error {
	for file, target := range linkMap {
		slog.InfoContext(context.Background(), "Creating a symlink", "file", file, "target", target)
		err := os.MkdirAll(filepath.Dir(file), 0770)
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
// as random access on storage devices performs better at a higher queue depth
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
func (s *S3manager) downloadFile(s3object *s3.Object, mkdirMutex *sync.Mutex) error {
	slog.InfoContext(context.Background(), "Downloading file from s3", "file", *s3object.Key)
	localObjectPath := filepath.Join(s.bucketLocalPath, *s3object.Key)

	// If one channel attempts to create a directory that already exists (this can actually happen)
	// then a permission denied error will be thrown. The mutex lock prevents this.
	mkdirMutex.Lock()
	err := os.MkdirAll(filepath.Dir(localObjectPath), 0770)
	mkdirMutex.Unlock()
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

	s.downloadedByteMutex.Lock()
	s.downloadedBytes += fileDownloadByteCount
	s.downloadedByteMutex.Unlock()

	err = os.Chtimes(localObjectPath, *s3object.LastModified, *s3object.LastModified)
	if err != nil {
		return trace.Wrap(err, "failed to update the access and modification time on file %q to %v", localObjectPath, *s3object.LastModified)
	}

	slog.InfoContext(context.Background(), "Download from s3 complete", "file", *s3object.Key, "size_bytes", fileDownloadByteCount)
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
		slog.DebugContext(context.Background(), "Starting to walk directory tree", "path", absPath)

		if err != nil {
			return trace.Wrap(err, "failed to walk over directory %q on path %q", s.bucketLocalPath)
		}

		syncFunc, err := s.syncGenericFsObject(absPath, info)
		if err != nil {
			return trace.Wrap(err, "failed to get syncing function for %q", absPath)
		}

		uploadGroup.Go(syncFunc)
		slog.DebugContext(context.Background(), "Upload of file queued", "file", absPath)
		return nil
	})

	// Even if an error has occurred we should wait to exit until all running syncs have
	// completed, even if not successful
	slog.InfoContext(context.Background(), "Waiting for sync to complete")
	syncErr := uploadGroup.Wait()
	// Future work: add upload logging information once
	// https://github.com/seqsense/s3sync/commit/29b3fcb259293d80634cb3916e0f28467d017087 has been released
	slog.InfoContext(context.Background(), "Sync has completed")

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
		slog.DebugContext(context.Background(), "Skipping non-empty directory")
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

	slog.InfoContext(context.Background(), "Creating a redirect file", "redirect_from", relSrcPath, "redirect_to", relTargetPath)
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
	slog.InfoContext(context.Background(), "Creating redirect", "redirect_from", remoteAbsSourcePath, "redirect_to", targetURL)

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
	if errors.Is(err, io.EOF) {
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

	slog.InfoContext(context.Background(), "Performing S3 sync", "source", src, "destination", dest)
	err := s.syncManager.Sync(src, dest)
	if err != nil {
		return trace.Wrap(err, "failed to sync %q to %q", src, dest)
	}
	slog.InfoContext(context.Background(), "S3 sync complete")

	return nil
}
