// Copyright 2023 Gravitational, Inc
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

package athena

import (
	"bytes"
	"context"
	"encoding/base64"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

const (
	payloadTypeAttr          = "payload_type"
	payloadTypeRawProtoEvent = "raw_proto_event"
	payloadTypeS3Based       = "s3_event"

	// maxSNSMessageSize defines maximum size of SNS message. AWS allows 256KB
	// however it counts also headers. We round it to 250KB, just to be sure.
	maxSNSMessageSize = 250 * 1024
	// maxS3BasedSize defines some resonable threshold for S3 based messages (2GB).
	maxS3BasedSize uint64 = 2 * 1024 * 1024 * 1024
)

// publisher is a SNS based events publisher.
// It publishes proto events directly to SNS topic, or use S3 bucket
// if payload is too large for SNS.
type publisher struct {
	PublisherConfig
}

type snsPublisher interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type s3uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type PublisherConfig struct {
	TopicARN      string
	SNSPublisher  snsPublisher
	Uploader      s3uploader
	PayloadBucket string
	PayloadPrefix string
}

// NewPublisher returns new instance of publisher.
func NewPublisher(cfg PublisherConfig) *publisher {
	return &publisher{
		PublisherConfig: cfg,
	}
}

// newPublisherFromAthenaConfig returns new instance of publisher from athena
// config.
func newPublisherFromAthenaConfig(cfg Config) *publisher {
	r := retry.NewStandard(func(so *retry.StandardOptions) {
		so.MaxAttempts = 20
		so.MaxBackoff = 1 * time.Minute
	})
	return NewPublisher(PublisherConfig{
		TopicARN: cfg.TopicARN,
		SNSPublisher: sns.NewFromConfig(*cfg.AWSConfig, func(o *sns.Options) {
			o.Retryer = r
		}),
		// TODO(tobiaszheller): consider reworking lib/observability to work also on s3 sdk-v2.
		Uploader:      manager.NewUploader(s3.NewFromConfig(*cfg.AWSConfig)),
		PayloadBucket: cfg.largeEventsBucket,
		PayloadPrefix: cfg.largeEventsPrefix,
	})
}

// EmitAuditEvent emits audit event to SNS topic. Topic should be connected with
// SQS via subscription, so event is persisted on queue.
// Events are marshaled as oneOf from apievents and encoded using base64.
// For large events, payload is publihsed to S3, and on SNS there is only passed
// location on S3.
func (p *publisher) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	// Just double check that audit event has minimum necessary fields for athena
	// to works. Teleport emitter layer above makes sure that they are filled.
	if in.GetID() == "" {
		return trace.BadParameter("missing uid of audit event %s", in.GetType())
	}
	if in.GetTime().IsZero() {
		return trace.BadParameter("missing time of audit event %s", in.GetType())
	}

	oneOf, err := apievents.ToOneOf(in)
	if err != nil {
		return trace.Wrap(err)
	}
	marshaledProto, err := oneOf.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	b64Encoded := base64.StdEncoding.EncodeToString(marshaledProto)
	if len(b64Encoded) > maxSNSMessageSize {
		if uint64(len(b64Encoded)) > maxS3BasedSize {
			return trace.BadParameter("message too large to publish, size %d", len(b64Encoded))
		}
		return trace.Wrap(p.emitViaS3(ctx, in.GetID(), marshaledProto))
	}
	return trace.Wrap(p.emitViaSNS(ctx, in.GetID(), b64Encoded))
}

func (p *publisher) emitViaS3(ctx context.Context, uid string, marshaledEvent []byte) error {
	path := filepath.Join(p.PayloadPrefix, uid)
	out, err := p.Uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.PayloadBucket),
		Key:    aws.String(path),
		Body:   bytes.NewBuffer(marshaledEvent),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var versionID string
	if out.VersionID != nil {
		versionID = *out.VersionID
	}
	msg := &apievents.AthenaS3EventPayload{
		Path:      path,
		VersionId: versionID,
	}
	buf, err := msg.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.SNSPublisher.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.TopicARN),
		Message:  aws.String(base64.StdEncoding.EncodeToString(buf)),
		MessageAttributes: map[string]snsTypes.MessageAttributeValue{
			payloadTypeAttr: {DataType: aws.String("String"), StringValue: aws.String(payloadTypeS3Based)},
		},
	})
	return trace.Wrap(err)
}

func (p *publisher) emitViaSNS(ctx context.Context, uid string, b64Encoded string) error {
	_, err := p.SNSPublisher.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.TopicARN),
		Message:  aws.String(b64Encoded),
		MessageAttributes: map[string]snsTypes.MessageAttributeValue{
			payloadTypeAttr: {DataType: aws.String("String"), StringValue: aws.String(payloadTypeRawProtoEvent)},
		},
	})
	return trace.Wrap(err)
}
