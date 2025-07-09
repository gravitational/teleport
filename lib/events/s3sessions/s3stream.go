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
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// CreateUpload creates a multipart upload
func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	start := time.Now()

	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(h.path(sessionID)),
	}
	if !h.Config.DisableServerSideEncryption {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms

		if h.Config.SSEKMSKey != "" {
			input.SSEKMSKeyId = aws.String(h.Config.SSEKMSKey)
		}
	}
	if h.Config.ACL != "" {
		input.ACL = types.ObjectCannedACL(h.Config.ACL)
	}

	resp, err := h.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return nil, trace.Wrap(awsutils.ConvertS3Error(err), "CreateMultiPartUpload session(%v)", sessionID)
	}

	h.logger.InfoContext(ctx, "Created upload",
		"duration", time.Since(start),
		"upload", aws.ToString(resp.UploadId),
		"session", sessionID,
		"key", aws.ToString(resp.Key),
	)

	return &events.StreamUpload{SessionID: sessionID, ID: aws.ToString(resp.UploadId)}, nil
}

// UploadPart uploads part
func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	// This upload exceeded maximum number of supported parts, error now.
	if partNumber > int64(s3manager.MaxUploadParts) {
		return nil, trace.LimitExceeded(
			"exceeded total allowed S3 limit MaxUploadParts (%d). Adjust PartSize to fit in this limit", s3manager.MaxUploadParts)
	}

	start := time.Now()
	uploadKey := h.path(upload.SessionID)
	log := h.logger.With(
		"upload", upload.ID,
		"session", upload.SessionID,
		"key", uploadKey,
	)

	// Calculate the content MD5 hash to be included in the request. This is required for S3 buckets with Object Lock enabled.
	hash := md5.New()
	if _, err := io.Copy(hash, partBody); err != nil {
		return nil, trace.Wrap(err, "failed to calculate content MD5 hash")
	}
	md5sum := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// Reset the partBody reader to the beginning before passing it the params.
	// This is necessary because after calculating the md5 hash the partBody reader will have been moved to the end of the data.
	if _, err := partBody.Seek(0, io.SeekStart); err != nil {
		return nil, trace.Wrap(err, "failed to reset part body reader to beginning")
	}

	params := &s3.UploadPartInput{
		Bucket:     aws.String(h.Bucket),
		UploadId:   aws.String(upload.ID),
		Key:        aws.String(uploadKey),
		Body:       partBody,
		PartNumber: aws.Int32(int32(partNumber)),
		ContentMD5: aws.String(md5sum),
	}

	log.DebugContext(ctx, "Uploading part", "part_number", partNumber)
	resp, err := h.client.UploadPart(ctx, params)
	if err != nil {
		return nil, trace.Wrap(awsutils.ConvertS3Error(err),
			"UploadPart(upload %v) part(%v) session(%v)", upload.ID, partNumber, upload.SessionID)
	}
	// TODO(espadolini): the AWS SDK v1 doesn't expose the Date of the response
	// in [s3.UploadPartOutput] so we use the current time instead; AWS SDK v2
	// might expose the returned Date as part of the metadata, so we should
	// check if that matches the actual LastModified of the part. It doesn't
	// make much sense to do an additional request to check the LastModified of
	// the part we just uploaded, however.
	log.InfoContext(ctx, "Uploaded part", "part_number", partNumber, "upload_curation", time.Since(start))
	return &events.StreamPart{
		ETag:         aws.ToString(resp.ETag),
		Number:       partNumber,
		LastModified: time.Now(),
	}, nil
}

func (h *Handler) abortUpload(ctx context.Context, upload events.StreamUpload) error {
	uploadKey := h.path(upload.SessionID)
	log := h.logger.With(
		"upload", upload.ID,
		"session", upload.SessionID,
		"key", uploadKey,
	)
	req := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(h.Bucket),
		Key:      aws.String(uploadKey),
		UploadId: aws.String(upload.ID),
	}
	log.DebugContext(ctx, "Aborting upload")
	_, err := h.client.AbortMultipartUpload(ctx, req)
	if err != nil {
		return awsutils.ConvertS3Error(err)
	}

	log.InfoContext(ctx, "Aborted upload")
	return nil
}

// maxPartsPerUpload is the maximum number of parts for a single multipart upload.
// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/qfacts.html
const maxPartsPerUpload = 10000

// CompleteUpload completes the upload
func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	if len(parts) == 0 {
		return h.abortUpload(ctx, upload)
	}
	if len(parts) > maxPartsPerUpload {
		return trace.BadParameter("too many parts for a single S3 upload (%d), "+
			"must be less than %d", len(parts), maxPartsPerUpload)
	}

	start := time.Now()
	uploadKey := h.path(upload.SessionID)
	log := h.logger.With(
		"upload", upload.ID,
		"session", upload.SessionID,
		"key", uploadKey,
	)

	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})

	completedParts := make([]types.CompletedPart, len(parts))
	for i := range parts {
		completedParts[i] = types.CompletedPart{
			ETag:       aws.String(parts[i].ETag),
			PartNumber: aws.Int32(int32(parts[i].Number)),
		}
	}

	log.DebugContext(ctx, "Completing upload")
	params := &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(h.Bucket),
		Key:             aws.String(uploadKey),
		UploadId:        aws.String(upload.ID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	}
	_, err := h.client.CompleteMultipartUpload(ctx, params)
	if err != nil {
		return trace.Wrap(awsutils.ConvertS3Error(err),
			"CompleteMultipartUpload(upload %v) session(%v)", upload.ID, upload.SessionID)
	}

	log.InfoContext(ctx, "Completed upload", "duration", time.Since(start))
	return nil
}

// ListParts lists upload parts
func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	uploadKey := h.path(upload.SessionID)
	log := h.logger.With(
		"upload", upload.ID,
		"session", upload.SessionID,
		"key", uploadKey,
	)
	log.DebugContext(ctx, "Listing parts for upload")

	var parts []events.StreamPart

	paginator := s3.NewListPartsPaginator(h.client, &s3.ListPartsInput{
		Bucket:   aws.String(h.Bucket),
		Key:      aws.String(uploadKey),
		UploadId: aws.String(upload.ID),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, awsutils.ConvertS3Error(err)
		}
		for _, part := range page.Parts {
			parts = append(parts, events.StreamPart{
				Number:       int64(aws.ToInt32(part.PartNumber)),
				ETag:         aws.ToString(part.ETag),
				LastModified: aws.ToTime(part.LastModified),
			})
		}
	}

	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})
	return parts, nil
}

// ListUploads lists uploads that have been initiated but not completed
func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	var prefix *string
	if h.Path != "" {
		trimmed := strings.TrimPrefix(h.Path, "/")
		prefix = &trimmed
	}
	var uploads []events.StreamUpload
	paginator := s3.NewListMultipartUploadsPaginator(h.client, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(h.Bucket),
		Prefix: prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, awsutils.ConvertS3Error(err)
		}
		for _, upload := range page.Uploads {
			if upload.Initiator != nil && upload.Initiator.DisplayName != nil && len(h.Config.CompleteInitiators) > 0 &&
				!slices.Contains(h.Config.CompleteInitiators, *upload.Initiator.DisplayName) {
				// Only complete uploads that we initiated.
				// This can be useful when Teleport is not the only thing generating uploads in the bucket
				// (replication rules, batch jobs, other software, etc.)
				continue
			}
			uploads = append(uploads, events.StreamUpload{
				ID:        aws.ToString(upload.UploadId),
				SessionID: h.fromPath(aws.ToString(upload.Key)),
				Initiated: aws.ToTime(upload.Initiated),
			})
		}
	}

	sort.Slice(uploads, func(i, j int) bool {
		return uploads[i].Initiated.Before(uploads[j].Initiated)
	})

	return uploads, nil
}

// GetUploadMetadata gets the metadata for session upload
func (h *Handler) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	sessionURL, err := url.JoinPath(teleport.SchemeS3+"://"+h.Bucket, h.Path, sessionID.String())
	if err != nil {
		// this should never happen, but if it does revert to legacy behavior
		// which omitted h.Path
		sessionURL = fmt.Sprintf("%v://%v/%v", teleport.SchemeS3, h.Bucket, sessionID)
	}

	return events.UploadMetadata{
		URL:       sessionURL,
		SessionID: sessionID,
	}
}

// ReserveUploadPart reserves an upload part.
func (h *Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}
