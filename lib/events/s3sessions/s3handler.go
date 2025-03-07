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
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	awsmetrics "github.com/gravitational/teleport/lib/observability/metrics/aws"
	s3metrics "github.com/gravitational/teleport/lib/observability/metrics/s3"
	"github.com/gravitational/teleport/lib/session"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/endpoint"
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
	// CredentialsProvider if supplied is used in tests or with External Audit Storage.
	CredentialsProvider aws.CredentialsProvider
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

	// IgnoreInitiator optionally configures the S3 uploader to ignore uploads
	// initiated by the specified party.
	IgnoreInitiator string
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

	if val := in.Query().Get(teleport.S3IgnoreInitiator); val != "" {
		s.IgnoreInitiator = val
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

	if s.Endpoint != "" {
		s.Endpoint = endpoint.CreateURI(s.Endpoint, s.Insecure)
	}

	return nil
}

// NewHandler returns new S3 uploader
func NewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	logger := slog.With(teleport.ComponentKey, teleport.SchemeS3)

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.Insecure {
		opts = append(opts, config.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	} else {
		hc, err := defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		opts = append(opts, config.WithHTTPClient(hc))
	}

	if cfg.CredentialsProvider != nil {
		opts = append(opts, config.WithCredentialsProvider(cfg.CredentialsProvider))
	}

	opts = append(opts,
		config.WithAPIOptions(awsmetrics.MetricsMiddleware()),
		config.WithAPIOptions(s3metrics.MetricsMiddleware()),
	)

	resolver, err := endpoint.NewLoggingResolver(
		s3.NewDefaultEndpointResolverV2(),
		logger.With(slog.Group("service",
			"id", s3.ServiceID,
			"api_version", s3.ServiceAPIVersion,
		)),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s3Opts := []func(*s3.Options){
		s3.WithEndpointResolverV2(resolver),
		func(o *s3.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		},
	}

	if cfg.Endpoint != "" {
		if _, err := url.Parse(cfg.Endpoint); err != nil {
			return nil, trace.BadParameter("configured S3 endpoint is invalid: %s", err.Error())
		}

		opts = append(opts, config.WithBaseEndpoint(cfg.Endpoint))

		s3Opts = append(s3Opts, func(options *s3.Options) {
			options.UsePathStyle = !cfg.UseVirtualStyleAddressing
		})
	}

	if modules.GetModules().IsBoringBinary() && cfg.UseFIPSEndpoint == types.ClusterAuditConfigSpecV2_FIPS_ENABLED {
		s3Opts = append(s3Opts, func(options *s3.Options) {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		})
	}

	awsConfig, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create S3 client with custom options
	client := s3.NewFromConfig(awsConfig, s3Opts...)

	uploader := manager.NewUploader(client)
	downloader := manager.NewDownloader(client)

	h := &Handler{
		logger:     logger,
		Config:     cfg,
		uploader:   uploader,
		downloader: downloader,
		client:     client,
	}

	start := time.Now()
	h.logger.InfoContext(ctx, "Setting up S3 bucket",
		"bucket", h.Bucket,
		"path", h.Path,
		"region", h.Region,
	)
	if err := h.ensureBucket(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	h.logger.InfoContext(ctx, "Setting up bucket S3 completed",
		"bucket", h.Bucket,
		"path", h.Path,
		"region", h.Region,
		"duration", time.Since(start),
	)
	return h, nil
}

// Handler handles upload and downloads to S3 object storage
type Handler struct {
	// Config is handler configuration
	Config
	// logger emits log messages
	logger     *slog.Logger
	uploader   *manager.Uploader
	downloader *manager.Downloader
	client     *s3.Client
}

// Close releases connection and resources associated with log if any
func (h *Handler) Close() error {
	return nil
}

// Upload uploads object to S3 bucket, reads the contents of the object from reader
// and returns the target S3 bucket path in case of successful upload.
func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	path := h.path(sessionID)

	uploadInput := &s3.PutObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(path),
		Body:   reader,
	}
	if !h.Config.DisableServerSideEncryption {
		uploadInput.ServerSideEncryption = awstypes.ServerSideEncryptionAwsKms
		if h.Config.SSEKMSKey != "" {
			uploadInput.SSEKMSKeyId = aws.String(h.Config.SSEKMSKey)
		}
	}
	if h.Config.ACL != "" {
		uploadInput.ACL = awstypes.ObjectCannedACL(h.Config.ACL)
	}
	_, err := h.uploader.Upload(ctx, uploadInput)
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

	h.logger.DebugContext(ctx, "Downloading recording from S3", "bucket", h.Bucket, "path", h.path(sessionID), "version_id", versionID)

	_, err = h.downloader.Download(ctx, writer, &s3.GetObjectInput{
		Bucket:    aws.String(h.Bucket),
		Key:       aws.String(h.path(sessionID)),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return awsutils.ConvertS3Error(err)
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

	paginator := s3.NewListObjectVersionsPaginator(h.client, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", awsutils.ConvertS3Error(err)
		}
		for _, v := range page.Versions {
			versions = append(versions, versionID{
				ID:        aws.ToString(v.VersionId),
				Timestamp: *v.LastModified,
			})
		}
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

// deleteBucket deletes the bucket and all its contents and is used in tests
func (h *Handler) deleteBucket(ctx context.Context) error {
	// first, list and delete all the objects in the bucket
	paginator := s3.NewListObjectVersionsPaginator(h.client, &s3.ListObjectVersionsInput{
		Bucket: aws.String(h.Bucket),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return awsutils.ConvertS3Error(err)
		}
		for _, ver := range page.Versions {
			_, err := h.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket:    aws.String(h.Bucket),
				Key:       ver.Key,
				VersionId: ver.VersionId,
			})
			if err != nil {
				return awsutils.ConvertS3Error(err)
			}
		}
	}

	_, err := h.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
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
	// Use a short timeout for the HeadBucket call in case it takes too long, in
	// #50747 this call would hang.
	shortCtx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
	defer cancel()
	_, err := h.client.HeadBucket(shortCtx, &s3.HeadBucketInput{
		Bucket: aws.String(h.Bucket),
	})
	err = awsutils.ConvertS3Error(err)
	switch {
	case err == nil:
		// assumes that bucket is administered by other entity
		return nil
	case trace.IsBadParameter(err):
		return trace.Wrap(err)
	case !trace.IsNotFound(err):
		h.logger.ErrorContext(ctx, "Failed to ensure that S3 bucket exists. This is expected if External Audit Storage is enabled or if Teleport has write-only access to the bucket, otherwise S3 session uploads may fail.", "bucket", h.Bucket, "error", err)
		return nil
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(h.Bucket),
		ACL:    awstypes.BucketCannedACLPrivate,
	}
	_, err = h.client.CreateBucket(ctx, input)
	if err := awsutils.ConvertS3Error(err); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		// if this client has not created the bucket, don't reconfigure it
		return nil
	}

	// Turn on versioning.
	_, err = h.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(h.Bucket),
		VersioningConfiguration: &awstypes.VersioningConfiguration{
			Status: awstypes.BucketVersioningStatusEnabled,
		},
	})
	if err := awsutils.ConvertS3Error(err); err != nil {
		return trace.Wrap(err, "failed to set versioning state for bucket %q", h.Bucket)
	}

	// Turn on server-side encryption for the bucket.
	if !h.DisableServerSideEncryption {
		_, err = h.client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
			Bucket: aws.String(h.Bucket),
			ServerSideEncryptionConfiguration: &awstypes.ServerSideEncryptionConfiguration{
				Rules: []awstypes.ServerSideEncryptionRule{
					{
						ApplyServerSideEncryptionByDefault: &awstypes.ServerSideEncryptionByDefault{
							SSEAlgorithm: awstypes.ServerSideEncryptionAwsKms,
						},
					},
				},
			},
		})
		if err := awsutils.ConvertS3Error(err); err != nil {
			return trace.Wrap(err, "failed to set encryption state for bucket %q", h.Bucket)
		}
	}
	return nil
}
