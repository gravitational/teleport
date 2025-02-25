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

package azsessions

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// sessionContainerParam and inprogressContainerParam are the parameters in the
// fragment that specify the containers to use for finalized session recordings
// and in-progress data.
const (
	sessionContainerParam    = "session_container"
	inprogressContainerParam = "inprogress_container"
)

// defaultSessionContainerName and defaultInprogressContainerName are the
// default container names for finalized session recordings and for in-progress
// data.
const (
	defaultSessionContainerName    = "session"
	defaultInprogressContainerName = "inprogress"
)

// sessionName returns the name of the blob that contains the recording for a
// given session.
func sessionName(sid session.ID) string {
	return sid.String()
}

// uploadMarkerPrefix is the prefix of the names of the upload marker blobs.
// Listing blobs with this prefix will return an empty blob for each upload.
const uploadMarkerPrefix = "upload/"

// uploadMarkerName returns the blob name for the marker for a given upload.
func uploadMarkerName(upload events.StreamUpload) string {
	return fmt.Sprintf("%v%v/%v", uploadMarkerPrefix, upload.SessionID, upload.ID)
}

// partPrefix returns the prefix for the upload part blobs for a given upload.
// Listing blobs with this prefix will return all the parts that currently make
// up the upload.
func partPrefix(upload events.StreamUpload) string {
	return fmt.Sprintf("part/%v/%v/", upload.SessionID, upload.ID)
}

// partName returns the name of the blob for a specific part in an upload.
func partName(upload events.StreamUpload, partNumber int64) string {
	return fmt.Sprintf("%v%v", partPrefix(upload), partNumber)
}

// field names used for logging
const (
	fieldSessionID  = "session_id"
	fieldUploadID   = "upload_id"
	fieldPartNumber = "part"
	fieldPartCount  = "parts"
)

// Config is a struct of parameters to define the behavior of Handler.
type Config struct {
	// ServiceURL is the URL for the storage account to use.
	ServiceURL url.URL

	// SessionContainerName is the name of the container that stores finalized
	// session recordings. Defaults to [defaultSessionContainerName].
	SessionContainerName string

	// InprogressContainerName is the name of the container that stores
	// in-progress data that's yet to be finalized in a recording. Defaults to
	// [defaultInprogressContainerName].
	InprogressContainerName string

	// Log is the logger to use. If unset, it will default to the global logger
	// with a component of "azblob".
	Log *slog.Logger
}

// SetFromURL sets values in Config based on the passed in URL: the fragment of
// the URL is parsed as if it was made out of query parameters, which define
// options for ourselves, and then the remainder of the URL is set as the
// service URL.
func (c *Config) SetFromURL(u *url.URL) error {
	if u == nil {
		return nil
	}

	c.ServiceURL = *u

	switch c.ServiceURL.Scheme {
	case teleport.SchemeAZBlob:
		c.ServiceURL.Scheme = "https"
	case teleport.SchemeAZBlobHTTP:
		c.ServiceURL.Scheme = "http"
	case "http", "https":
	default:
		return trace.BadParameter("unsupported URL scheme %v", c.ServiceURL.Scheme)
	}

	params, err := url.ParseQuery(c.ServiceURL.EscapedFragment())
	if err != nil {
		return trace.Wrap(err)
	}
	c.ServiceURL.Fragment = ""
	c.ServiceURL.RawFragment = ""

	c.SessionContainerName = params.Get(sessionContainerParam)
	c.InprogressContainerName = params.Get(inprogressContainerParam)

	return nil
}

func (c *Config) CheckAndSetDefaults() error {
	if c.SessionContainerName == "" {
		c.SessionContainerName = defaultSessionContainerName
	}

	if c.InprogressContainerName == "" {
		c.InprogressContainerName = defaultInprogressContainerName
	}

	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, "azblob")
	}

	return nil
}

func NewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating Azure credentials")
	}

	ensureContainer := func(name string) (*container.Client, error) {
		containerURL := cfg.ServiceURL
		containerURL.Path = name

		cntClient, err := container.NewClient(containerURL.String(), cred, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		_, err = cErr(cntClient.GetProperties(ctx, nil))
		if err == nil {
			return cntClient, nil
		}
		if !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}

		cfg.Log.DebugContext(ctx, "Failed to confirm that the container exists, attempting creation.",
			"container", name,
			"error", err,
		)
		// someone else might've created the container between GetProperties and
		// Create, so we ignore AlreadyExists
		_, err = cErr(cntClient.Create(ctx, nil))
		if err == nil || trace.IsAlreadyExists(err) {
			return cntClient, nil
		}
		if trace.IsAccessDenied(err) {
			// we might not have permissions to read the container or to create
			// it, but we might have permissions to use it
			cfg.Log.WarnContext(ctx,
				"Could not create container, please ensure it exists or session recordings will not be stored correctly.",
				"container", name,
				"error", err,
			)
			return cntClient, nil
		}
		return nil, trace.Wrap(err)
	}

	session, err := ensureContainer(cfg.SessionContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inprogress, err := ensureContainer(cfg.InprogressContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		log:        cfg.Log,
		cred:       cred,
		session:    session,
		inprogress: inprogress,
	}, nil
}

// Handler is a MultipartHandler that stores data in Azure Blob Storage.
type Handler struct {
	log        *slog.Logger
	cred       azcore.TokenCredential
	session    *container.Client
	inprogress *container.Client
}

var _ events.MultipartHandler = (*Handler)(nil)

// sessionBlob returns a BlockBlobClient for the blob of the recording of the
// session.
func (h *Handler) sessionBlob(sessionID session.ID) *blockblob.Client {
	return h.session.NewBlockBlobClient(sessionName(sessionID))
}

// uploadMarkerBlob returns a BlockBlobClient for the marker blob of the stream
// upload.
func (h *Handler) uploadMarkerBlob(upload events.StreamUpload) *blockblob.Client {
	return h.inprogress.NewBlockBlobClient(uploadMarkerName(upload))
}

// partBlob returns a BlockBlobClient for the blob of the part of the specified
// upload, with the given part number.
func (h *Handler) partBlob(upload events.StreamUpload, partNumber int64) *blockblob.Client {
	return h.inprogress.NewBlockBlobClient(partName(upload, partNumber))
}

// Upload implements [events.UploadHandler].
func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	sessionBlob := h.sessionBlob(sessionID)

	if _, err := cErr(sessionBlob.UploadStream(ctx, reader, &blockblob.UploadStreamOptions{
		AccessConditions: &blobDoesNotExist,
	})); err != nil {
		return "", trace.Wrap(err)
	}
	h.log.DebugContext(ctx, "Uploaded session.", fieldSessionID, sessionID)

	return sessionBlob.URL(), nil
}

// Download implements [events.UploadHandler].
func (h *Handler) Download(ctx context.Context, sessionID session.ID, writerAt io.WriterAt) error {
	resp, err := cErr(h.sessionBlob(sessionID).DownloadStream(ctx, nil))
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.log.WarnContext(ctx, "Error closing downloaded session blob.", "error", err, fieldSessionID, sessionID)
		}
	}()

	writer, ok := writerAt.(io.Writer)
	if !ok {
		writer = io.NewOffsetWriter(writerAt, 0)
	}

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return trace.ConvertSystemError(cErr0(err))
	}

	h.log.DebugContext(ctx, "Downloaded session.", fieldSessionID, sessionID)
	return nil
}

// CreateUpload implements [events.MultipartUploader].
func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	upload := events.StreamUpload{
		ID:        uuid.NewString(),
		SessionID: sessionID,
	}

	if _, err := cErr(h.uploadMarkerBlob(upload).Upload(ctx, nil, &blockblob.UploadOptions{
		AccessConditions: &blobDoesNotExist,
	})); err != nil {
		return nil, trace.Wrap(err)
	}
	h.log.DebugContext(ctx, "Created upload marker.", fieldSessionID, sessionID)

	return &upload, nil
}

// CompleteUpload implements [events.MultipartUploader] by composing the final
// session recording blob in the session container from the parts in the
// inprogress container, using the Put Block From URL API. Might take a little
// time, but doesn't require any data transfer.
func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	sessionBlob := h.sessionBlob(upload.SessionID)

	// TODO(espadolini): explore the possibility of using leases to get
	// exclusive access while writing, and to guarantee that leftover parts are
	// cleaned up before a new attempt

	parts = slices.Clone(parts)
	slices.SortFunc(parts, func(a, b events.StreamPart) int { return cmp.Compare(a.Number, b.Number) })

	partURLs := make([]string, 0, len(parts))
	for _, part := range parts {
		partURLs = append(partURLs, h.partBlob(upload, part.Number).URL())
	}

	token, err := h.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	if err != nil {
		return trace.Wrap(err, "obtaining Azure authentication token")
	}
	copySourceAuthorization := "Bearer " + token.Token
	stageOptions := &blockblob.StageBlockFromURLOptions{
		CopySourceAuthorization: &copySourceAuthorization,
	}

	log := h.log.With(
		fieldSessionID, upload.SessionID,
		fieldUploadID, upload.ID,
		fieldPartCount, len(parts),
	)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(5) // default parallelism as used by azblob.DoBatchTransfer

	log.DebugContext(ctx, "Beginning upload completion.")
	blockNames := make([]string, len(parts))
	// TODO(espadolini): use stable names (upload id, part number and then some
	// hash maybe) to avoid re-staging parts more than once across multiple
	// completion attempts?
	for i := range parts {
		ii := i
		eg.Go(func() error {
			// we use block names that are local to this function so we don't
			// interact with other ongoing uploads; trick copied from
			// (*BlockBlobClient).UploadBuffer and UploadFile
			u := uuid.New()
			blockNames[ii] = base64.StdEncoding.EncodeToString(u[:])

			if _, err := cErr(sessionBlob.StageBlockFromURL(egCtx, blockNames[ii], partURLs[ii], stageOptions)); err != nil {
				return trace.Wrap(err)
			}
			log.DebugContext(egCtx, "Staged part.", fieldPartNumber, ii)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Committing part list.")
	if _, err := cErr(sessionBlob.CommitBlockList(ctx, blockNames, &blockblob.CommitBlockListOptions{
		AccessConditions: &blobDoesNotExist,
	})); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		log.WarnContext(ctx, "Session upload already exists, cleaning up marker.")
		parts = nil // don't delete parts that we didn't persist
	} else {
		log.DebugContext(ctx, "Completed session upload.")
	}

	// TODO(espadolini): should the cleanup run in its own goroutine? What
	// should the cancellation context for the cleanup be in that case?
	if _, err := cErr(h.uploadMarkerBlob(upload).Delete(ctx, nil)); err != nil && !trace.IsNotFound(err) {
		log.WarnContext(ctx, "Failed to clean up upload marker.", "error", err, fieldPartCount, len(parts))
		return nil
	}

	const batchSize = 256 // https://learn.microsoft.com/en-us/rest/api/storageservices/blob-batch
	for i := 0; i < len(parts); i += batchSize {
		batch, err := cErr(h.inprogress.NewBatchBuilder())
		if err != nil {
			return trace.Wrap(err)
		}

		m := batchSize
		if len(parts[i:]) < batchSize {
			m = len(parts[i:])
		}

		for _, part := range parts[i : i+m] {
			if err := batch.Delete(partName(upload, part.Number), nil); err != nil {
				return trace.Wrap(err)
			}
		}

		resp, err := cErr(h.inprogress.SubmitBatch(ctx, batch, nil))
		if err != nil {
			log.WarnContext(ctx, "Failed to clean up part batch.", "error", err, fieldPartNumber, parts[i].Number)
			continue
		}

		errs := 0
		for _, r := range resp.Responses {
			if r.Error != nil {
				err = r.Error
				errs++
			}
		}
		if errs > 0 {
			log.WarnContext(ctx, "Failed to clean up part batch.",
				fieldPartNumber, parts[i].Number,
				"errors", errs,
				"last_error", err,
			)
		}
	}

	return nil
}

// ReserveUploadPart implements [events.MultipartUploader].
func (*Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

// UploadPart implements [events.MultipartUploader].
func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	partBlob := h.partBlob(upload, partNumber)

	// our parts are just over 5 MiB (events.MinUploadPartSizeBytes) so we can
	// upload them in one shot
	response, err := cErr(partBlob.Upload(ctx, streaming.NopCloser(partBody), nil))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.log.DebugContext(ctx, "Uploaded part.",
		fieldSessionID, upload.SessionID,
		fieldUploadID, upload.ID,
		fieldPartNumber, partNumber,
	)

	var lastModified time.Time
	if response.LastModified != nil {
		lastModified = *response.LastModified
	}
	return &events.StreamPart{Number: partNumber, LastModified: lastModified}, nil
}

// ListParts implements [events.MultipartUploader].
func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	prefix := partPrefix(upload)

	var parts []events.StreamPart
	pager := h.inprogress.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		resp, err := cErr(pager.NextPage(ctx))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if resp.Segment == nil {
			continue
		}
		parts = slices.Grow(parts, len(resp.Segment.BlobItems))
		for _, b := range resp.Segment.BlobItems {
			if b == nil ||
				b.Name == nil ||
				!strings.HasPrefix(*b.Name, prefix) {
				continue
			}

			pn := strings.TrimPrefix(*b.Name, prefix)
			partNumber, err := strconv.ParseInt(pn, 10, 64)
			if err != nil {
				continue
			}
			var lastModified time.Time
			if b.Properties != nil &&
				b.Properties.LastModified != nil {
				lastModified = *b.Properties.LastModified
			}
			parts = append(parts, events.StreamPart{
				Number:       partNumber,
				LastModified: lastModified,
			})
		}
	}

	slices.SortFunc(parts, func(a, b events.StreamPart) int { return cmp.Compare(a.Number, b.Number) })

	return parts, nil
}

// ListUploads implements [events.MultipartUploader].
func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	prefix := uploadMarkerPrefix
	var uploads []events.StreamUpload

	pager := h.inprogress.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		r, err := cErr(pager.NextPage(ctx))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if r.Segment == nil {
			continue
		}
		uploads = slices.Grow(uploads, len(r.Segment.BlobItems))
		for _, b := range r.Segment.BlobItems {
			if b == nil ||
				b.Name == nil ||
				!strings.HasPrefix(*b.Name, prefix) ||
				b.Properties == nil ||
				b.Properties.CreationTime == nil {
				continue
			}

			name := strings.TrimPrefix(*b.Name, prefix)
			sid, uid, ok := strings.Cut(name, "/")
			if !ok {
				continue
			}
			if _, err := session.ParseID(sid); err != nil {
				continue
			}
			if _, err := uuid.Parse(uid); err != nil {
				continue
			}

			uploads = append(uploads, events.StreamUpload{
				ID:        uid,
				SessionID: session.ID(sid),
				Initiated: *b.Properties.CreationTime,
			})
		}
	}

	slices.SortFunc(uploads, func(a, b events.StreamUpload) int { return a.Initiated.Compare(b.Initiated) })

	return uploads, nil
}

// GetUploadMetadata implements [events.MultipartUploader].
func (h *Handler) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	return events.UploadMetadata{
		URL:       h.sessionBlob(sessionID).URL(),
		SessionID: sessionID,
	}
}
