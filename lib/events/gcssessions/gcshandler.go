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

package gcssessions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/session"
)

var (
	uploadRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gcs_event_storage_uploads",
			Help: "Number of uploads to the GCS backend",
		},
	)
	downloadRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gcs_event_storage_downloads",
			Help: "Number of downloads from the GCS backend",
		},
	)
	uploadLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "gcs_event_storage_uploads_seconds",
			Help: "Latency for GCS upload operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	downloadLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "gcs_event_storage_downloads_seconds",
			Help: "Latency for GCS download operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)

	prometheusCollectors = []prometheus.Collector{uploadRequests, downloadRequests, uploadLatencies, downloadLatencies}
)

const (
	// endpointPropertyKey
	endpointPropertyKey = "endpoint"
	// credentialsPath is used to supply credentials to teleport via JSON-typed service account key file
	credentialsPath = "credentialsPath"
	// projectID is used to to lookup GCS resources for a given GCP project
	projectID = "projectID"
	// kmsKeyName
	kmsKeyName = "keyName"
	// pathPropertyKey
	pathPropertyKey = "path"
)

// Config is handler configuration
type Config struct {
	// Bucket is GCS bucket name
	Bucket string
	// Path is an optional bucket path
	Path string
	// Path to the credentials file
	CredentialsPath string
	// The GCS project ID
	ProjectID string
	// KMS key name
	KMSKeyName string
	// Endpoint
	Endpoint string
	// OnComposerRun is used for fault injection in tests
	// runs (or doesn't run composer and returns error
	OnComposerRun func(ctx context.Context, composer *storage.Composer) (*storage.ObjectAttrs, error)
	// AfterObjectDelete is used for fault injection in tests
	// runs (or doesn't run object delete) and returns error
	AfterObjectDelete func(ctx context.Context, object *storage.ObjectHandle, error error) error
}

// SetFromURL sets values on the Config from the supplied URI
func (cfg *Config) SetFromURL(url *url.URL) error {
	kmsKeyNameParamString := url.Query().Get(kmsKeyName)
	if len(kmsKeyNameParamString) > 0 {
		cfg.KMSKeyName = kmsKeyNameParamString
	}

	endpointParamString := url.Query().Get(endpointPropertyKey)
	if len(endpointParamString) > 0 {
		cfg.Endpoint = endpointParamString
	}

	pathParamString := url.Query().Get(pathPropertyKey)
	if len(pathParamString) > 0 {
		cfg.Path = pathParamString
	}

	credentialsPathParamString := url.Query().Get(credentialsPath)
	if len(credentialsPathParamString) > 0 {
		cfg.CredentialsPath = credentialsPathParamString
	}

	projectIDParamString := url.Query().Get(projectID)
	if projectIDParamString == "" {
		return trace.BadParameter("parameter %s with value '%s' is invalid",
			projectID, projectIDParamString)
	}
	cfg.ProjectID = projectIDParamString

	if url.Host == "" {
		return trace.BadParameter("host should be set to the bucket name for recording storage")
	}
	cfg.Bucket = url.Host

	return nil
}

// CheckAndSetDefaults checks and sets default values
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.OnComposerRun == nil {
		cfg.OnComposerRun = composerRun
	}
	if cfg.AfterObjectDelete == nil {
		cfg.AfterObjectDelete = afterObjectDelete
	}
	return nil
}

// afterObjectDelete is a passthrough function to delete an object
func afterObjectDelete(ctx context.Context, object *storage.ObjectHandle, err error) error {
	return err
}

// ComposerRun is a passthrough function that runs composer
func composerRun(ctx context.Context, composer *storage.Composer) (*storage.ObjectAttrs, error) {
	return composer.Run(ctx)
}

// DefaultNewHandler returns a new handler with default GCS client settings derived from the config
func DefaultNewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	var args []option.ClientOption
	if len(cfg.Endpoint) != 0 {
		args = append(args, option.WithoutAuthentication(), option.WithEndpoint(cfg.Endpoint), option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	} else if len(cfg.CredentialsPath) != 0 {
		args = append(args, option.WithCredentialsFile(cfg.CredentialsPath))
	}

	ctx, cancelFunc := context.WithCancel(ctx)
	client, err := storage.NewClient(ctx, args...)
	if err != nil {
		cancelFunc()
		return nil, trace.Wrap(convertGCSError(err), "error creating GCS gcsClient")
	}

	return NewHandler(ctx, cancelFunc, cfg, client)
}

// NewHandler returns a new handler with specific context, cancelFunc, and client
func NewHandler(ctx context.Context, cancelFunc context.CancelFunc, cfg Config, client *storage.Client) (*Handler, error) {
	err := metrics.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	h := &Handler{
		logger:        slog.With(teleport.ComponentKey, teleport.SchemeGCS),
		Config:        cfg,
		gcsClient:     client,
		clientContext: ctx,
		clientCancel:  cancelFunc,
	}
	start := time.Now()
	h.logger.InfoContext(ctx, "Setting up GCS bucket.", "bucket", h.Bucket, "path", h.Path)
	if err := h.ensureBucket(); err != nil {
		return nil, trace.Wrap(err)
	}
	h.logger.InfoContext(ctx, "Setting up bucket completed.", "bucket", h.Bucket, "duration", time.Since(start))
	return h, nil
}

// Handler handles upload and downloads to GCS object storage
type Handler struct {
	// Config is handler configuration
	Config
	// logger emits log messages
	logger *slog.Logger
	// gcsClient is the google cloud storage client used for persistence
	gcsClient *storage.Client
	// clientContext is used for non-request operations and cleanup
	clientContext context.Context
	// clientCancel is a function that will cancel the clientContext
	clientCancel context.CancelFunc
}

// Close releases connection and resources associated with log if any
func (h *Handler) Close() error {
	h.clientCancel()
	return h.gcsClient.Close()
}

// Upload uploads object to GCS bucket, reads the contents of the object from reader
// and returns the target GCS bucket path in case of successful upload.
func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	path := h.path(sessionID)
	h.logger.DebugContext(ctx, "Uploading object to GCS", "path", path)

	// Make sure we don't overwrite an existing recording.
	_, err := h.gcsClient.Bucket(h.Config.Bucket).Object(path).Attrs(ctx)
	if !errors.Is(err, storage.ErrObjectNotExist) {
		if err != nil {
			return "", convertGCSError(err)
		}
		return "", trace.AlreadyExists("recording for session %q already exists in GCS", sessionID)
	}

	writer := h.gcsClient.Bucket(h.Config.Bucket).Object(path).NewWriter(ctx)
	start := time.Now()
	_, err = io.Copy(writer, reader)
	// Always close the writer, even if upload failed.
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	uploadLatencies.Observe(time.Since(start).Seconds())
	uploadRequests.Inc()
	if err != nil {
		return "", convertGCSError(err)
	}
	return fmt.Sprintf("%v://%v/%v", teleport.SchemeGCS, h.Bucket, path), nil
}

// Download downloads recorded session from GCS bucket and writes the results into writer
// return trace.NotFound error is object is not found
func (h *Handler) Download(ctx context.Context, sessionID session.ID, writerAt io.WriterAt) error {
	path := h.path(sessionID)
	h.logger.DebugContext(ctx, "Downloading object from GCS.", "path", path)
	writer, ok := writerAt.(io.Writer)
	if !ok {
		return trace.BadParameter("the provided writerAt is %T which does not implement io.Writer", writerAt)
	}
	reader, err := h.gcsClient.Bucket(h.Config.Bucket).Object(path).NewReader(ctx)
	if err != nil {
		return convertGCSError(err)
	}
	defer reader.Close()
	start := time.Now()
	written, err := io.Copy(writer, reader)
	if err != nil {
		return convertGCSError(err)
	}
	downloadLatencies.Observe(time.Since(start).Seconds())
	downloadRequests.Inc()
	if written == 0 {
		return trace.NotFound("recording for %v is empty", sessionID)
	}
	return nil
}

func (h *Handler) path(sessionID session.ID) string {
	if h.Path == "" {
		return string(sessionID) + ".tar"
	}
	return strings.TrimPrefix(path.Join(h.Path, string(sessionID)+".tar"), slash)
}

// ensureBucket makes sure bucket exists, and if it does not, creates it
// this app should not have the authority to create/destroy resources
func (h *Handler) ensureBucket() error {
	_, err := h.gcsClient.Bucket(h.Config.Bucket).Attrs(h.clientContext)
	err = convertGCSError(err)
	// assumes that bucket is administered by other entity
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		h.logger.ErrorContext(h.clientContext,
			"Failed to ensure that bucket exists. GCS session uploads may fail. If you've set up the bucket already and gave Teleport write-only access, feel free to ignore this error.",
			"bucket", h.Bucket, "error", err)
		return nil
	}
	err = h.gcsClient.Bucket(h.Config.Bucket).Create(h.clientContext, h.Config.ProjectID, &storage.BucketAttrs{
		VersioningEnabled: true,
		Encryption:        &storage.BucketEncryption{DefaultKMSKeyName: h.Config.KMSKeyName},
		// See https://cloud.google.com/storage/docs/json_api/v1/buckets/insert#parameters
		PredefinedACL:              "projectPrivate",
		PredefinedDefaultObjectACL: "projectPrivate",
	})
	err = convertGCSError(err)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// if this gcsClient has not created the bucket, don't reconfigure it
		return nil
	}
	return nil
}

func convertGCSError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, storage.ErrBucketNotExist), errors.Is(err, storage.ErrObjectNotExist):
		return trace.NotFound("%s", err)
	default:
		return trace.Wrap(err)
	}
}
