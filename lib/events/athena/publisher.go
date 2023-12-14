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
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

const (
	payloadTypeAttr          = "payload_type"
	payloadTypeRawProtoEvent = "raw_proto_event"
	payloadTypeS3Based       = "s3_event"

	// maxSNSMessageSize defines maximum size of SNS message. AWS allows 256KB
	// however it counts also headers. We round it to 250KB, just to be sure.
	maxSNSMessageSize = 250 * 1024
)

var (
	// maxS3BasedSize defines some resonable threshold for S3 based messages
	// (almost 2GiB but fits in an int).
	//
	// It's a var instead of const so tests can override it instead of casually
	// allocating 2GiB.
	maxS3BasedSize = 2*1024*1024*1024 - 1
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
		SNSPublisher: sns.NewFromConfig(*cfg.PublisherConsumerAWSConfig, func(o *sns.Options) {
			o.Retryer = r
		}),
		// TODO(tobiaszheller): consider reworking lib/observability to work also on s3 sdk-v2.
		Uploader:      manager.NewUploader(s3.NewFromConfig(*cfg.PublisherConsumerAWSConfig)),
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
	ctx = context.WithoutCancel(ctx)
	// Teleport emitter layer above makes sure that they are filled.
	// We fill it just to be sure in case some problems with layer above, it's
	// better to generate it, then skip event.
	if in.GetID() == "" {
		in.SetID(uuid.NewString())
	}
	if in.GetTime().IsZero() {
		in.SetTime(time.Now().UTC().Round(time.Millisecond))
	}

	// Attempt to trim the event to maxS3BasedSize. This is a no-op if the event
	// is already small enough. If it can not be trimmed or the event is still
	// too large after marshaling then we may fail to emit the event below.
	//
	// This limit is much larger than events.MaxEventBytesInResponse and the
	// event may need to be trimmed again on the querier side, but this is an
	// attempt to preserve as much of the event as possible in case we add the
	// ability to query very large events in the future.
	if t, ok := in.(trimmableEvent); ok {
		prevSize := in.Size()
		// Trim to 3/4 the max size because base64 has 33% overhead.
		// The TrimToMaxSize implementations have a 10% buffer already.
		in = t.TrimToMaxSize(maxS3BasedSize - maxS3BasedSize/4)
		if in.Size() != prevSize {
			events.MetricStoredTrimmedEvents.Inc()
		}
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
		if len(b64Encoded) > maxS3BasedSize {
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
