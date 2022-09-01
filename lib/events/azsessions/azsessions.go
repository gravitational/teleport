// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azsessions

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

const (
	sessionContainerName    = "session"
	inprogressContainerName = "inprogress"

	// uploadMarkerPrefix is the prefix for upload markers, stored at `upload/<session ID>/<upload ID>`.
	uploadMarkerPrefix = "upload/"
	// uploadMarkerFmt is the format string for upload markers, stored at `upload/<session ID>/<upload ID>`.
	uploadMarkerFmt = "upload/%v/%v"

	// partFmt is the format string for upload parts, stored at `part/<session ID>/<upload ID>/<part number>`.
	partFmt = "part/%v/%v/%v"

	// clientIDFragParam is the parameter in the fragment that specifies the optional client ID.
	clientIDFragParam = "azure_client_id"
)

// field names used for logging
const (
	fieldSessionID  = "session_id"
	fieldUploadID   = "upload_id"
	fieldPartNumber = "part"
	fieldPartCount  = "parts"
)

type Config struct {
	// ServiceURL is the URL for the storage account to use.
	ServiceURL url.URL

	// ClientID, when set, defines the managed identity's client ID to use for
	// authentication.
	ClientID string

	// Log is the logger to use. If unset, it will default to the global logger
	// with a component of "azblob".
	Log logrus.FieldLogger
}

func (c *Config) SetFromURL(u *url.URL) error {
	c.ServiceURL = *u

	switch c.ServiceURL.Scheme {
	case teleport.SchemeAZBlob:
		c.ServiceURL.Scheme = "https"
	case teleport.SchemeAZBlobHTTP:
		c.ServiceURL.Scheme = "http"
	}

	params, err := url.ParseQuery(c.ServiceURL.EscapedFragment())
	if err != nil {
		return err
	}
	c.ServiceURL.Fragment = ""
	c.ServiceURL.RawFragment = ""

	c.ClientID = params.Get(clientIDFragParam)

	return nil
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.SchemeAZBlob)
	}

	return nil
}

func NewHandlerFromURL(ctx context.Context, u *url.URL) (*Handler, error) {
	var cfg Config
	if err := cfg.SetFromURL(u); err != nil {
		return nil, err
	}
	return NewHandler(ctx, cfg)
}

func NewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	var cred azcore.TokenCredential
	if cfg.ClientID != "" {
		c, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(cfg.ClientID),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	} else {
		c, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	}

	cred = &cachedTokenCredential{TokenCredential: cred}

	service, err := azblob.NewServiceClient(cfg.ServiceURL.String(), cred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := service.NewContainerClient(sessionContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inprogress, err := service.NewContainerClient(inprogressContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := cErr2(session.GetProperties(ctx, nil)); err != nil {
		if !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
			return nil, err
		}
		cfg.Log.WithError(err).Debugf("Failed to confirm that the %v container exists, attempting creation.", sessionContainerName)
		// someone else might've created the container between GetProperties and
		// Create, so we ignore AlreadyExists
		if _, err := cErr2(session.Create(ctx, nil)); err != nil && !trace.IsAlreadyExists(err) {
			if !trace.IsAccessDenied(err) {
				return nil, err
			}
			cfg.Log.WithError(err).Warnf(
				"Could not create the %v container, please ensure it exists or session recordings will not be stored correctly.", sessionContainerName)
		}
	}

	if _, err := cErr2(inprogress.GetProperties(ctx, nil)); err != nil {
		if !trace.IsNotFound(err) {
			return nil, err
		}
		cfg.Log.WithError(err).Debugf("Failed to confirm that the %v container exists, attempting creation.", inprogressContainerName)
		// someone else might've created the container between GetProperties and
		// Create, so we ignore AlreadyExists
		if _, err := cErr2(inprogress.Create(ctx, nil)); err != nil && !trace.IsAlreadyExists(err) {
			if !trace.IsAccessDenied(err) {
				return nil, err
			}
			cfg.Log.WithError(err).Warnf(
				"Could not create the %v container, please ensure it exists or session recordings will not be stored correctly.", inprogressContainerName)
		}
	}

	return &Handler{c: cfg, cred: cred, session: session, inprogress: inprogress}, nil
}

type Handler struct {
	c          Config
	cred       azcore.TokenCredential
	session    *azblob.ContainerClient
	inprogress *azblob.ContainerClient
}

var _ events.MultipartHandler = (*Handler)(nil)

// sessionBlob returns a BlockBlobClient for the blob of the recording of the
// session. Not expected to ever fail.
func (h *Handler) sessionBlob(sessionID session.ID) (*azblob.BlockBlobClient, error) {
	blobName := sessionID.String()
	client, err := h.session.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// uploadMarkerBlob returns a BlockBlobClient for the marker blob of the stream
// upload. Not expected to ever fail.
func (h *Handler) uploadMarkerBlob(upload events.StreamUpload) (*azblob.BlockBlobClient, error) {
	blobName := fmt.Sprintf(uploadMarkerFmt, upload.SessionID, upload.ID)
	client, err := h.inprogress.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// partBlob returns a BlockBlobClient for the blob of the part of the specified
// upload, with the given part number. Not expected to ever fail.
func (h *Handler) partBlob(upload events.StreamUpload, partNumber int64) (*azblob.BlockBlobClient, error) {
	blobName := fmt.Sprintf(partFmt, upload.SessionID, upload.ID, partNumber)
	client, err := h.inprogress.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	blob, err := h.sessionBlob(sessionID)
	if err != nil {
		return "", err
	}

	if _, err := cErr2(blob.UploadStream(ctx, reader, azblob.UploadStreamOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		return "", err
	}
	h.c.Log.WithField(fieldSessionID, sessionID).Debug("Uploaded session.")

	return blob.URL(), nil
}

func (h *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	blob, err := h.sessionBlob(sessionID)
	if err != nil {
		return err
	}

	const beginOffset = 0
	if err := cErr(blob.DownloadToWriterAt(ctx, beginOffset, azblob.CountToEnd, writer, azblob.DownloadOptions{})); err != nil {
		return err
	}
	h.c.Log.WithField(fieldSessionID, sessionID).Debug("Downloaded session.")

	return nil
}

func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	upload := events.StreamUpload{
		ID:        uuid.NewString(),
		SessionID: sessionID,
	}

	blob, err := h.uploadMarkerBlob(upload)
	if err != nil {
		return nil, err
	}

	emptyBody := streaming.NopCloser(&bytes.Reader{})
	if _, err := cErr2(blob.Upload(ctx, emptyBody, &azblob.BlockBlobUploadOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		return nil, err
	}
	h.c.Log.WithField(fieldSessionID, sessionID).Debug("Created upload marker.")

	return &upload, nil
}

func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	blob, err := h.sessionBlob(upload.SessionID)
	if err != nil {
		return err
	}

	// TODO(espadolini): explore the possibility of using leases to get
	// exclusive access while writing, and to guarantee that leftover parts are
	// cleaned up before a new attempt

	parts = slices.Clone(parts)
	slices.SortFunc(parts, func(a, b events.StreamPart) bool { return a.Number < b.Number })

	partURLs := make([]string, 0, len(parts))
	for _, part := range parts {
		b, err := h.partBlob(upload, part.Number)
		if err != nil {
			return err
		}
		partURLs = append(partURLs, b.URL())
	}

	token, err := h.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	copySourceAuthorization := "Bearer " + token.Token
	stageOptions := &azblob.BlockBlobStageBlockFromURLOptions{
		CopySourceAuthorization: &copySourceAuthorization,
	}

	log := h.c.Log.WithFields(logrus.Fields{
		fieldSessionID: upload.SessionID,
		fieldUploadID:  upload.ID,
	})

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(5) // default parallelism as used by azblob.DoBatchTransfer

	log.WithField(fieldPartCount, len(parts)).Debug("Beginning upload completion.")
	blockNames := make([]string, len(parts))
	// TODO(espadolini): use stable names (upload id, part number and then some
	// hash maybe) to avoid re-staging parts more than once across multiple
	// completion attempts?
	for i := range parts {
		i := i
		eg.Go(func() error {
			// we use block names that are local to this function so we don't
			// interact with other ongoing uploads; trick copied from
			// (*BlockBlobClient).UploadBuffer and UploadFile
			u := uuid.New()
			blockNames[i] = base64.StdEncoding.EncodeToString(u[:])

			const contentLength = 0 // required by the API to be zero
			if _, err := cErr2(blob.StageBlockFromURL(egCtx, blockNames[i], partURLs[i], contentLength, stageOptions)); err != nil {
				return err
			}
			log.WithField(fieldPartNumber, i).Debug("Staged part.")
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	log.Debug("Committing part list.")
	if _, err := cErr2(blob.CommitBlockList(ctx, blockNames, &azblob.BlockBlobCommitBlockListOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		if !trace.IsAlreadyExists(err) {
			return err
		}
		log.Warn("Session upload already exists, cleaning up marker.")
		parts = nil // don't delete parts that we didn't persist
	} else {
		log.Debug("Completed session upload.")
	}

	// TODO(espadolini): should the cleanup run in its own goroutine? What
	// should the cancellation context for the cleanup be in that case?
	markerBlob, err := h.uploadMarkerBlob(upload)
	if err != nil {
		log.Warn("Failed to clean up upload marker.")
		return nil
	}
	if _, err := cErr2(markerBlob.Delete(ctx, nil)); err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to clean up upload marker.")
		return nil
	}

	// TODO(espadolini): group deletes together with Blob Batch, not supported
	// by the SDK
	for _, part := range parts {
		b, err := h.partBlob(upload, part.Number)
		if err != nil {
			log.WithField(fieldPartNumber, part.Number).Warn("Failed to clean up part.")
		}
		if _, err := cErr2(b.Delete(ctx, nil)); err != nil {
			log.WithField(fieldPartNumber, part.Number).WithError(err).Warn("Failed to clean up part.")
		}
	}

	return nil
}

func (*Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	blob, err := h.partBlob(upload, partNumber)
	if err != nil {
		return nil, err
	}

	// our parts are just over 5 MiB (events.MinUploadPartSizeBytes) so we can
	// upload them in one shot
	if _, err := cErr2(blob.Upload(ctx, streaming.NopCloser(partBody), nil)); err != nil {
		return nil, err
	}
	h.c.Log.WithFields(logrus.Fields{
		fieldSessionID:  upload.SessionID,
		fieldUploadID:   upload.ID,
		fieldPartNumber: partNumber,
	}).Debug("Uploaded part.")

	return &events.StreamPart{Number: partNumber}, nil
}

func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	prefix := fmt.Sprintf(partFmt, upload.SessionID, upload.ID, "")

	var parts []events.StreamPart
	pager := h.inprogress.ListBlobsFlat(&azblob.ContainerListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.NextPage(ctx) {
		resp := pager.PageResponse()
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
			partNumber, err := strconv.ParseInt(pn, 10, 0)
			if err != nil {
				continue
			}

			parts = append(parts, events.StreamPart{Number: partNumber})
		}
	}
	if err := cErr(pager.Err()); err != nil {
		return nil, err
	}

	slices.SortFunc(parts, func(a, b events.StreamPart) bool { return a.Number < b.Number })

	return parts, nil
}

func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	prefix := uploadMarkerPrefix
	var uploads []events.StreamUpload

	pager := h.inprogress.ListBlobsFlat(&azblob.ContainerListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.NextPage(ctx) {
		r := pager.PageResponse()
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
	if err := cErr(pager.Err()); err != nil {
		return nil, err
	}

	slices.SortFunc(uploads, func(a, b events.StreamUpload) bool { return a.Initiated.Before(b.Initiated) })

	return uploads, nil
}

func (h *Handler) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	url := h.c.ServiceURL
	url.Path = path.Join(url.Path, sessionContainerName, sessionID.String())

	return events.UploadMetadata{
		URL:       url.String(),
		SessionID: sessionID,
	}
}
