/*
Copyright 2020 Gravitational, Inc.

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

package s3sessions

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
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
		input.ServerSideEncryption = aws.String(s3.ServerSideEncryptionAwsKms)

		if h.Config.SSEKMSKey != "" {
			input.SSEKMSKeyId = aws.String(h.Config.SSEKMSKey)
		}
	}
	if h.Config.ACL != "" {
		input.ACL = aws.String(h.Config.ACL)
	}

	resp, err := h.client.CreateMultipartUploadWithContext(ctx, input)
	if err != nil {
		return nil, trace.Wrap(awsutils.ConvertS3Error(err), "CreateMultiPartUpload session(%v)", sessionID)
	}

	h.WithFields(logrus.Fields{
		"upload":  aws.StringValue(resp.UploadId),
		"session": sessionID,
		"key":     aws.StringValue(resp.Key),
	}).Infof("Created upload in %v", time.Since(start))

	return &events.StreamUpload{SessionID: sessionID, ID: aws.StringValue(resp.UploadId)}, nil
}

// UploadPart uploads part
func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	// This upload exceeded maximum number of supported parts, error now.
	if partNumber > s3manager.MaxUploadParts {
		return nil, trace.LimitExceeded(
			"exceeded total allowed S3 limit MaxUploadParts (%d). Adjust PartSize to fit in this limit", s3manager.MaxUploadParts)
	}

	start := time.Now()
	uploadKey := h.path(upload.SessionID)
	log := h.WithFields(logrus.Fields{
		"upload":  upload.ID,
		"session": upload.SessionID,
		"key":     uploadKey,
	})

	params := &s3.UploadPartInput{
		Bucket:     aws.String(h.Bucket),
		UploadId:   aws.String(upload.ID),
		Key:        aws.String(uploadKey),
		Body:       partBody,
		PartNumber: aws.Int64(partNumber),
	}

	log.Debugf("Uploading part %v", partNumber)
	resp, err := h.client.UploadPartWithContext(ctx, params)
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
	log.Infof("Uploaded part %v in %v", partNumber, time.Since(start))
	return &events.StreamPart{
		ETag:         aws.StringValue(resp.ETag),
		Number:       partNumber,
		LastModified: time.Now(),
	}, nil
}

func (h *Handler) abortUpload(ctx context.Context, upload events.StreamUpload) error {
	uploadKey := h.path(upload.SessionID)
	log := h.WithFields(logrus.Fields{
		"upload":  upload.ID,
		"session": upload.SessionID,
		"key":     uploadKey,
	})
	req := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(h.Bucket),
		Key:      aws.String(uploadKey),
		UploadId: aws.String(upload.ID),
	}
	log.Debug("Aborting upload")
	_, err := h.client.AbortMultipartUploadWithContext(ctx, req)
	if err != nil {
		return awsutils.ConvertS3Error(err)
	}

	log.Info("Aborted upload")
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
	log := h.WithFields(logrus.Fields{
		"upload":  upload.ID,
		"session": upload.SessionID,
		"key":     uploadKey,
	})

	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})

	completedParts := make([]*s3.CompletedPart, len(parts))
	for i := range parts {
		completedParts[i] = &s3.CompletedPart{
			ETag:       aws.String(parts[i].ETag),
			PartNumber: aws.Int64(parts[i].Number),
		}
	}

	log.Debug("Completing upload")
	params := &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(h.Bucket),
		Key:             aws.String(uploadKey),
		UploadId:        aws.String(upload.ID),
		MultipartUpload: &s3.CompletedMultipartUpload{Parts: completedParts},
	}
	_, err := h.client.CompleteMultipartUploadWithContext(ctx, params)
	if err != nil {
		return trace.Wrap(awsutils.ConvertS3Error(err),
			"CompleteMultipartUpload(upload %v) session(%v)", upload.ID, upload.SessionID)
	}

	log.Infof("Completed upload in %v", time.Since(start))
	return nil
}

// ListParts lists upload parts
func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	uploadKey := h.path(upload.SessionID)
	log := h.WithFields(logrus.Fields{
		"upload":  upload.ID,
		"session": upload.SessionID,
		"key":     uploadKey,
	})
	log.Debug("Listing parts for upload")

	var parts []events.StreamPart
	var partNumberMarker *int64
	for i := 0; i < defaults.MaxIterationLimit; i++ {
		re, err := h.client.ListPartsWithContext(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(h.Bucket),
			Key:              aws.String(uploadKey),
			UploadId:         aws.String(upload.ID),
			PartNumberMarker: partNumberMarker,
		})
		if err != nil {
			return nil, awsutils.ConvertS3Error(err)
		}
		for _, part := range re.Parts {
			parts = append(parts, events.StreamPart{
				Number:       aws.Int64Value(part.PartNumber),
				ETag:         aws.StringValue(part.ETag),
				LastModified: aws.TimeValue(part.LastModified),
			})
		}
		if !aws.BoolValue(re.IsTruncated) {
			break
		}
		partNumberMarker = re.NextPartNumberMarker
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
	var keyMarker *string
	var uploadIDMarker *string
	for i := 0; i < defaults.MaxIterationLimit; i++ {
		input := &s3.ListMultipartUploadsInput{
			Bucket:         aws.String(h.Bucket),
			Prefix:         prefix,
			KeyMarker:      keyMarker,
			UploadIdMarker: uploadIDMarker,
		}
		re, err := h.client.ListMultipartUploadsWithContext(ctx, input)
		if err != nil {
			return nil, awsutils.ConvertS3Error(err)
		}
		for _, upload := range re.Uploads {
			uploads = append(uploads, events.StreamUpload{
				ID:        aws.StringValue(upload.UploadId),
				SessionID: h.fromPath(aws.StringValue(upload.Key)),
				Initiated: aws.TimeValue(upload.Initiated),
			})
		}
		if !aws.BoolValue(re.IsTruncated) {
			break
		}
		keyMarker = re.NextKeyMarker
		uploadIDMarker = re.NextUploadIdMarker
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
