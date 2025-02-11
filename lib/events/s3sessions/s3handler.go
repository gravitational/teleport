/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package s3sessions

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	s3metrics "github.com/gravitational/teleport/lib/observability/metrics/s3"
	"github.com/gravitational/teleport/lib/session"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// s3AllowedACL is the set of canned ACLs that S3 accepts
var s3AllowedACL = map[string]struct{}{
	"private":                   {},
	"public-read":               {},
	"public-read-write":         {},
	"aws-exec-read":             {},
	"authenticated-read":        {},
	"bucket-owner-read":         {},
	"bucket-owner-full-control": {},
	"log-delivery-write":        {},
}

func isCannedACL(acl string) bool {
	_, ok := s3AllowedACL[acl]
	return ok
}

// Config is handler configuration
type Config struct {
	// Bucket is S3 bucket name
	Bucket string
	// Region is S3 bucket region
	Region string
	// Path is an optional bucket path
	Path string
	// Endpoint is an optional third party S3 compatible endpoint
	Endpoint string
	// ACL is the canned ACL to send to S3
	ACL string
	// Session is an optional existing AWS client session
	Session *awssession.Session
	// Credentials if supplied are used in tests or with External Audit Storage.
	Credentials *credentials.Credentials
	// SSEKMSKey specifies the optional custom CMK used for KMS SSE.
	SSEKMSKey string

	// UseFIPSEndpoint uses AWS FedRAMP/FIPS 140-2 mode endpoints.
	// to determine its behavior:
	// Unset - allows environment variables or AWS config to set the value
	// Enabled - explicitly enabled
	// Disabled - explicitly disabled
	UseFIPSEndpoint types.ClusterAuditConfigSpecV2_FIPSEndpointState

	// Insecure is an optional switch to opt out of https connections
	Insecure bool
	// DisableServerSideEncryption is an optional switch to opt out of SSE in case the provider does not support it
	DisableServerSideEncryption bool

	// UseVirtualStyleAddressing use a virtual-hosted–style URI.
	// Path style e.g. https://s3.region-code.amazonaws.com/bucket-name/key-name
	// Virtual hosted style e.g. https://bucket-name.s3.region-code.amazonaws.com/key-name
	// Teleport defaults to path-style addressing for better interoperability
	// with 3rd party S3-compatible services out of the box.
	// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html for more details.
	UseVirtualStyleAddressing bool
}

// SetFromURL sets values on the Config from the supplied URI
func (s *Config) SetFromURL(in *url.URL, inRegion string) error {
	region := inRegion
	if uriRegion := in.Query().Get(teleport.Region); uriRegion != "" {
		region = uriRegion
	}
	if endpoint := in.Query().Get(teleport.Endpoint); endpoint != "" {
		s.Endpoint = endpoint
	}

	const boolErrorTemplate = "failed to parse URI %q flag %q - %q, supported values are 'true', 'false', or any other" +
		"supported boolean in https://pkg.go.dev/strconv#ParseBool"
	if val := in.Query().Get(teleport.Insecure); val != "" {
		insecure, err := strconv.ParseBool(val)
		if err != nil {
			return trace.BadParameter(boolErrorTemplate, in.String(), teleport.Insecure, val)
		}
		s.Insecure = insecure
	}
	if val := in.Query().Get(teleport.DisableServerSideEncryption); val != "" {
		disableServerSideEncryption, err := strconv.ParseBool(val)
		if err != nil {
			return trace.BadParameter(boolErrorTemplate, in.String(), teleport.DisableServerSideEncryption, val)
		}
		s.DisableServerSideEncryption = disableServerSideEncryption
	}
	if acl := in.Query().Get(teleport.ACL); acl != "" {
		if !isCannedACL(acl) {
			return trace.BadParameter("failed to parse URI %q flag %q - %q is not a valid canned ACL", in.String(), teleport.ACL, acl)
		}
		s.ACL = acl
	}
	if val := in.Query().Get(teleport.SSEKMSKey); val != "" {
		s.SSEKMSKey = val
	}

	if val := in.Query().Get(events.UseFIPSQueryParam); val != "" {
		useFips, err := strconv.ParseBool(val)
		if err != nil {
			return trace.BadParameter(boolErrorTemplate, in.String(), events.UseFIPSQueryParam, val)
		}
		if useFips {
			s.UseFIPSEndpoint = types.ClusterAuditConfigSpecV2_FIPS_ENABLED
		} else {
			s.UseFIPSEndpoint = types.ClusterAuditConfigSpecV2_FIPS_DISABLED
		}
	}

	if val := in.Query().Get(teleport.S3UseVirtualStyleAddressing); val != "" {
		useVirtualStyleAddressing, err := strconv.ParseBool(val)
		if err != nil {
			return trace.BadParameter(boolErrorTemplate, in.String(), teleport.S3UseVirtualStyleAddressing, val)
		}
		s.UseVirtualStyleAddressing = useVirtualStyleAddressing
	} else {
		// Default to false for backwards compatibility
		s.UseVirtualStyleAddressing = false
	}

	s.Region = region
	s.Bucket = in.Host
	s.Path = in.Path
	return nil
}

// CheckAndSetDefaults checks and sets defaults
func (s *Config) CheckAndSetDefaults() error {
	if s.Bucket == "" {
		return trace.BadParameter("missing parameter Bucket")
	}
	if s.Session == nil {
		awsConfig := aws.Config{
			UseFIPSEndpoint: events.FIPSProtoStateToAWSState(s.UseFIPSEndpoint),
		}
		if s.Region != "" {
			awsConfig.Region = aws.String(s.Region)
		}
		if s.Endpoint != "" {
			awsConfig.Endpoint = aws.String(s.Endpoint)
			awsConfig.S3ForcePathStyle = aws.Bool(true)
		}
		if s.Insecure {
			awsConfig.DisableSSL = aws.Bool(s.Insecure)
		}
		if s.Credentials != nil {
			awsConfig.Credentials = s.Credentials
		}
		usePathStyle := !s.UseVirtualStyleAddressing
		awsConfig.S3ForcePathStyle = &usePathStyle
		hc, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		awsConfig.HTTPClient = hc

		sess, err := awssession.NewSessionWithOptions(awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
			Config:            awsConfig,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		s.Session = sess
	}
	return nil
}

// NewHandler returns new S3 uploader
func NewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := s3metrics.NewAPIMetrics(s3.New(cfg.Session))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uploader, err := s3metrics.NewUploadAPIMetrics(s3manager.NewUploader(cfg.Session))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	downloader, err := s3metrics.NewDownloadAPIMetrics(s3manager.NewDownloader(cfg.Session))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		Entry: log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.Component(teleport.SchemeS3),
		}),
		Config:     cfg,
		uploader:   uploader,
		downloader: downloader,
		client:     client,
	}
	start := time.Now()
	h.Infof("Setting up bucket %q, sessions path %q in region %q.", h.Bucket, h.Path, h.Region)
	if err := h.ensureBucket(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	h.WithFields(log.Fields{"duration": time.Since(start)}).Infof("Setup bucket %q completed.", h.Bucket)
	return h, nil
}

// Handler handles upload and downloads to S3 object storage
type Handler struct {
	// Config is handler configuration
	Config
	// Entry is a logging entry
	*log.Entry
	uploader   s3manageriface.UploaderAPI
	downloader s3manageriface.DownloaderAPI
	client     s3iface.S3API
}

// Close releases connection and resources associated with log if any
func (h *Handler) Close() error {
	return nil
}

// Upload uploads object to S3 bucket, reads the contents of the object from reader
// and returns the target S3 bucket path in case of successful upload.
func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	var err error
	path := h.path(sessionID)

	uploadInput := &s3manager.UploadInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(path),
		Body:   reader,
	}
	if !h.Config.DisableServerSideEncryption {
		uploadInput.ServerSideEncryption = aws.String(s3.ServerSideEncryptionAwsKms)

		if h.Config.SSEKMSKey != "" {
			uploadInput.SSEKMSKeyId = aws.String(h.Config.SSEKMSKey)
		}
	}
	if h.Config.ACL != "" {
		uploadInput.ACL = aws.String(h.Config.ACL)
	}
	_, err = h.uploader.UploadWithContext(ctx, uploadInput)
	if err != nil {
		return "", awsutils.ConvertS3Error(err)
	}
	return fmt.Sprintf("%v://%v/%v", teleport.SchemeS3, h.Bucket, path), nil
}

// Download downloads recorded session from S3 bucket and writes the results
// into writer return trace.NotFound error is object is not found.
func (h *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	// Get the oldest version of this object. This has to be done because S3
	// allows overwriting objects in a bucket. To prevent corruption of recording
	// data, get all versions and always return the first.
	versionID, err := h.getOldestVersion(ctx, h.Bucket, h.path(sessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	h.Debugf("Downloading %v/%v [%v].", h.Bucket, h.path(sessionID), versionID)

	written, err := h.downloader.DownloadWithContext(ctx, writer, &s3.GetObjectInput{
		Bucket:    aws.String(h.Bucket),
		Key:       aws.String(h.path(sessionID)),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return awsutils.ConvertS3Error(err)
	}
	if written == 0 {
		return trace.NotFound("recording for %v is not found", sessionID)
	}
	return nil
}

// versionID is used to store versions of a key to allow sorting by timestamp.
type versionID struct {
	// ID is the version ID.
	ID string

	// Timestamp is the last time the object was modified.
	Timestamp time.Time
}

// getOldestVersion returns the oldest version of the object.
func (h *Handler) getOldestVersion(ctx context.Context, bucket string, prefix string) (string, error) {
	var versions []versionID

	// Get all versions of this object.
	err := h.client.ListObjectVersionsPagesWithContext(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectVersionsOutput, lastPage bool) bool {
		for _, v := range page.Versions {
			versions = append(versions, versionID{
				ID:        *v.VersionId,
				Timestamp: *v.LastModified,
			})
		}

		// Returning false stops iteration, stop iteration upon last page.
		return !lastPage
	})
	if err != nil {
		return "", awsutils.ConvertS3Error(err)
	}
	if len(versions) == 0 {
		return "", trace.NotFound("%v/%v not found", bucket, prefix)
	}

	// Sort the versions slice so the first entry is the oldest and return it.
	sort.Slice(versions, func(i int, j int) bool {
		return versions[i].Timestamp.Before(versions[j].Timestamp)
	})
	return versions[0].ID, nil
}

// delete bucket deletes bucket and all it's contents and is used in tests
func (h *Handler) deleteBucket(ctx context.Context) error {
	// first, list and delete all the objects in the bucket
	out, err := h.client.ListObjectVersionsWithContext(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(h.Bucket),
	})
	if err != nil {
		return awsutils.ConvertS3Error(err)
	}
	for _, ver := range out.Versions {
		_, err := h.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
			Bucket:    aws.String(h.Bucket),
			Key:       ver.Key,
			VersionId: ver.VersionId,
		})
		if err != nil {
			return awsutils.ConvertS3Error(err)
		}
	}
	_, err = h.client.DeleteBucketWithContext(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(h.Bucket),
	})
	return awsutils.ConvertS3Error(err)
}

func (h *Handler) path(sessionID session.ID) string {
	if h.Path == "" {
		return string(sessionID) + ".tar"
	}
	return strings.TrimPrefix(path.Join(h.Path, string(sessionID)+".tar"), "/")
}

func (h *Handler) fromPath(p string) session.ID {
	return session.ID(strings.TrimSuffix(path.Base(p), ".tar"))
}

// ensureBucket makes sure bucket exists, and if it does not, creates it
func (h *Handler) ensureBucket(ctx context.Context) error {
	_, err := h.client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(h.Bucket),
	})
	err = awsutils.ConvertS3Error(err)
	// assumes that bucket is administered by other entity
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		h.Errorf("Failed to ensure that bucket %q exists (%v). S3 session uploads may fail. If you've set up the bucket already and gave Teleport write-only access, feel free to ignore this error.", h.Bucket, err)
		return nil
	}
	input := &s3.CreateBucketInput{
		Bucket: aws.String(h.Bucket),
		ACL:    aws.String("private"),
	}
	_, err = h.client.CreateBucketWithContext(ctx, input)
	err = awsutils.ConvertS3Error(err, fmt.Sprintf("bucket %v already exists", aws.String(h.Bucket)))
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// if this client has not created the bucket, don't reconfigure it
		return nil
	}

	// Turn on versioning.
	ver := &s3.PutBucketVersioningInput{
		Bucket: aws.String(h.Bucket),
		VersioningConfiguration: &s3.VersioningConfiguration{
			Status: aws.String("Enabled"),
		},
	}
	_, err = h.client.PutBucketVersioningWithContext(ctx, ver)
	err = awsutils.ConvertS3Error(err, fmt.Sprintf("failed to set versioning state for bucket %q", h.Bucket))
	if err != nil {
		return trace.Wrap(err)
	}

	// Turn on server-side encryption for the bucket.
	if !h.DisableServerSideEncryption {
		_, err = h.client.PutBucketEncryptionWithContext(ctx, &s3.PutBucketEncryptionInput{
			Bucket: aws.String(h.Bucket),
			ServerSideEncryptionConfiguration: &s3.ServerSideEncryptionConfiguration{
				Rules: []*s3.ServerSideEncryptionRule{{
					ApplyServerSideEncryptionByDefault: &s3.ServerSideEncryptionByDefault{
						SSEAlgorithm: aws.String(s3.ServerSideEncryptionAwsKms),
					},
				}},
			},
		})
		err = awsutils.ConvertS3Error(err, fmt.Sprintf("failed to set versioning state for bucket %q", h.Bucket))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
