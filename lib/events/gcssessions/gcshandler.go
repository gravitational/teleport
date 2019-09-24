package gcssessions

/*

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

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/session"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
)

func init() {
	prometheus.MustRegister(uploadRequests)
	prometheus.MustRegister(downloadRequests)
	prometheus.MustRegister(uploadLatencies)
	prometheus.MustRegister(downloadLatencies)
}

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
	} else {
		cfg.ProjectID = projectIDParamString
	}

	if url.Host == "" {
		return trace.BadParameter("host should be set to the bucket name for recording storage")
	} else {
		cfg.Bucket = url.Host
	}

	return nil
}

// DefaultNewHandler returns a new handler with default GCS client settings derived from the config
func DefaultNewHandler(cfg Config) (*Handler, error) {

	var args []option.ClientOption
	if len(cfg.Endpoint) != 0 {
		args = append(args, option.WithoutAuthentication(), option.WithEndpoint(cfg.Endpoint), option.WithGRPCDialOption(grpc.WithInsecure()))
	} else if len(cfg.CredentialsPath) != 0 {
		args = append(args, option.WithCredentialsFile(cfg.CredentialsPath))
	}

	ctx, cancelFuc := context.WithCancel(context.Background())

	client, err := storage.NewClient(ctx, args...)

	if err != nil {
		return nil, trace.BadParameter("error creating GCS gcsClient; error %v", convertGCSError(err))
	}

	return NewHandler(ctx, cancelFuc, cfg, client)
}

// NewHandler returns a new handler with specific context, cancelFunc, and client
func NewHandler(ctx context.Context, cancelFunc context.CancelFunc, cfg Config, client *storage.Client) (*Handler, error) {
	h := &Handler{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.SchemeGCS),
		}),
		Config:        cfg,
		gcsClient:     client,
		clientContext: ctx,
		clientCancel:  cancelFunc,
	}
	start := time.Now()
	h.Infof("Setting up bucket %q, sessions path %q.", h.Bucket, h.Path)
	if err := h.ensureBucket(); err != nil {
		return nil, trace.Wrap(err)
	}
	h.WithFields(log.Fields{"duration": time.Now().Sub(start)}).Infof("Setup bucket %q completed.", h.Bucket)
	return h, nil
}

// Handler handles upload and downloads to GCS object storage
type Handler struct {
	// Config is handler configuration
	Config
	// Entry is a logging entry
	*log.Entry
	// gcsClient is the google cloud storage client used for persistence
	gcsClient *storage.Client
	// clientContext is used for non-request operations and cleanup
	clientContext context.Context
	// clientCancel is a function that will cancel the clientContext
	clientCancel context.CancelFunc
}

// Closer releases connection and resources associated with log if any
func (h *Handler) Close() error {
	h.clientCancel()
	return h.gcsClient.Close()
}

// Upload uploads object to GCS bucket, reads the contents of the object from reader
// and returns the target GCS bucket path in case of successful upload.
func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	path := h.path(sessionID)
	h.Logger.Debugf("uploading %s", path)
	writer := h.gcsClient.Bucket(h.Config.Bucket).Object(path).NewWriter(ctx)
	start := time.Now()
	_, err := io.Copy(writer, reader)
	if err == nil {
		err = writer.Close()
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
func (h *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	path := h.path(sessionID)
	h.Logger.Debugf("downloading %s", path)
	reader, err := h.gcsClient.Bucket(h.Config.Bucket).Object(path).NewReader(ctx)
	if err != nil {
		return convertGCSError(err)
	}
	start := time.Now()
	written, err := io.Copy(writer.(io.Writer), reader)
	if err != nil {
		return convertGCSError(err)
	}
	downloadLatencies.Observe(time.Since(start).Seconds())
	downloadRequests.Inc()
	if written == 0 {
		return trace.NotFound("recording for %v is not found", sessionID)
	}
	return nil
}

// delete bucket deletes bucket and all it's contents and is used in tests
// this app should not have the authority to create/destroy resources
func (h *Handler) deleteBucket() error {
	objectsIterator := h.gcsClient.Bucket(h.Config.Bucket).Objects(h.clientContext, nil)
	for {
		attrs, err := objectsIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return convertGCSError(err)
		}
		err = h.gcsClient.Bucket(h.Config.Bucket).Object(attrs.Name).Delete(h.clientContext)
		if err != nil {
			return convertGCSError(err)
		}
	}
	err := h.gcsClient.Bucket(h.Config.Bucket).Delete(h.clientContext)
	return convertGCSError(err)
}

func (h *Handler) path(sessionID session.ID) string {
	if h.Path == "" {
		return string(sessionID) + ".tar"
	}
	return strings.TrimPrefix(filepath.Join(h.Path, string(sessionID)+".tar"), "/")
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
		return trace.Wrap(err)
	}
	err = h.gcsClient.Bucket(h.Config.Bucket).Create(h.clientContext, h.Config.ProjectID, &storage.BucketAttrs{
		VersioningEnabled: true,
		Encryption:        &storage.BucketEncryption{DefaultKMSKeyName: h.Config.KMSKeyName},
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

func convertGCSError(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}

	switch err {
	case storage.ErrBucketNotExist, storage.ErrObjectNotExist:
		return trace.NotFound(err.Error(), args...)
	default:
		return trace.BadParameter(err.Error(), args...)
	}
}
