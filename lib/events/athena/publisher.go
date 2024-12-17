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
	"net/http"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsratelimit "github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	awsretry "github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
)

const (
	payloadTypeAttr          = "payload_type"
	payloadTypeRawProtoEvent = "raw_proto_event"
	payloadTypeS3Based       = "s3_event"

	// maxDirectMessageSize defines maximum size of SNS/SQS message. AWS allows
	// 256KB however it counts also headers. We round it to 250KB, just to be
	// sure.
	maxDirectMessageSize = 250 * 1024
)

// maxS3BasedSize defines some resonable threshold for S3 based messages
// (almost 2GiB but fits in an int).
//
// It's a var instead of const so tests can override it instead of casually
// allocating 2GiB.
var maxS3BasedSize = 2*1024*1024*1024 - 1

// publisher is a SNS based events publisher.
// It publishes proto events directly to SNS topic, or use S3 bucket
// if payload is too large for SNS.
type publisher struct {
	PublisherConfig
}

type messagePublisher interface {
	// Publish sends a message with a given body to a notification topic or a
	// queue (or something similar), with added metadata to signify whether or
	// not the message is only a reference to a S3 object or a full message.
	Publish(ctx context.Context, base64Body string, s3Based bool) error
}

type messagePublisherFunc func(ctx context.Context, base64Body string, s3Based bool) error

// Publish implements [messagePublisher].
func (f messagePublisherFunc) Publish(ctx context.Context, base64Body string, s3Based bool) error {
	return f(ctx, base64Body, s3Based)
}

// SNSPublisherFunc returns a message publisher that sends messages to a SNS
// topic through the given SNS client.
func SNSPublisherFunc(topicARN string, snsClient *sns.Client) messagePublisherFunc {
	return func(ctx context.Context, base64Body string, s3Based bool) error {
		var messageAttributes map[string]snstypes.MessageAttributeValue
		if s3Based {
			messageAttributes = map[string]snstypes.MessageAttributeValue{
				payloadTypeAttr: {
					DataType:    aws.String("String"),
					StringValue: aws.String(payloadTypeS3Based),
				},
			}
		} else {
			messageAttributes = map[string]snstypes.MessageAttributeValue{
				payloadTypeAttr: {
					DataType:    aws.String("String"),
					StringValue: aws.String(payloadTypeRawProtoEvent),
				},
			}
		}

		_, err := snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn:          &topicARN,
			Message:           &base64Body,
			MessageAttributes: messageAttributes,
		})
		return trace.Wrap(err)
	}
}

// SQSPublisherFunc returns a message publisher that sends messages to a SQS
// queue through the given SQS client.
func SQSPublisherFunc(queueURL string, sqsClient *sqs.Client) messagePublisherFunc {
	return func(ctx context.Context, base64Body string, s3Based bool) error {
		var messageAttributes map[string]sqstypes.MessageAttributeValue
		if s3Based {
			messageAttributes = map[string]sqstypes.MessageAttributeValue{
				payloadTypeAttr: {
					DataType:    aws.String("String"),
					StringValue: aws.String(payloadTypeS3Based),
				},
			}
		} else {
			messageAttributes = map[string]sqstypes.MessageAttributeValue{
				payloadTypeAttr: {
					DataType:    aws.String("String"),
					StringValue: aws.String(payloadTypeRawProtoEvent),
				},
			}
		}

		_, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:          &queueURL,
			MessageBody:       &base64Body,
			MessageAttributes: messageAttributes,
		})
		return trace.Wrap(err)
	}
}

type s3uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type PublisherConfig struct {
	MessagePublisher messagePublisher
	Uploader         s3uploader
	PayloadBucket    string
	PayloadPrefix    string
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
	r := awsretry.NewStandard(func(so *awsretry.StandardOptions) {
		so.MaxAttempts = 20
		so.MaxBackoff = 1 * time.Minute
		// failure to do an API call likely means that we've just lost data, so
		// let's just have the server bounce us back repeatedly rather than give
		// up in the client
		so.RateLimiter = awsratelimit.None
	})
	hc := awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		// aggressively reuse connections for the sake of avoiding TLS
		// handshakes (the default MaxIdleConnsPerHost is a pitiful 2)
		t.MaxIdleConns = defaults.HTTPMaxIdleConns
		t.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost
	})
	var messagePublisher messagePublisherFunc
	if cfg.TopicARN == topicARNBypass {
		messagePublisher = SQSPublisherFunc(cfg.QueueURL, sqs.NewFromConfig(*cfg.PublisherConsumerAWSConfig, func(o *sqs.Options) {
			o.Retryer = r
			o.HTTPClient = hc
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		}))
	} else {
		messagePublisher = SNSPublisherFunc(cfg.TopicARN, sns.NewFromConfig(*cfg.PublisherConsumerAWSConfig, func(o *sns.Options) {
			o.Retryer = r
			o.HTTPClient = hc
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		}))
	}

	return NewPublisher(PublisherConfig{
		MessagePublisher: messagePublisher,
		Uploader: s3manager.NewUploader(s3.NewFromConfig(*cfg.PublisherConsumerAWSConfig, func(o *s3.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		})),
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
	prevSize := in.Size()
	// Trim to 3/4 the max size because base64 has 33% overhead.
	// The TrimToMaxSize implementations have a 10% buffer already.
	in = in.TrimToMaxSize(maxS3BasedSize - maxS3BasedSize/4)
	if in.Size() != prevSize {
		events.MetricStoredTrimmedEvents.Inc()
	}

	oneOf, err := apievents.ToOneOf(in)
	if err != nil {
		return trace.Wrap(err)
	}
	marshaledProto, err := oneOf.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	base64Len := base64.StdEncoding.EncodedLen(len(marshaledProto))
	if base64Len > maxDirectMessageSize {
		if base64Len > maxS3BasedSize {
			return trace.BadParameter("message too large to publish, size %d", base64Len)
		}
		return trace.Wrap(p.emitViaS3(ctx, in.GetID(), marshaledProto))
	}
	base64Body := base64.StdEncoding.EncodeToString(marshaledProto)
	const s3BasedFalse = false
	return trace.Wrap(p.MessagePublisher.Publish(ctx, base64Body, s3BasedFalse))
}

func (p *publisher) emitViaS3(ctx context.Context, uid string, marshaledEvent []byte) error {
	path := path.Join(p.PayloadPrefix, uid)
	out, err := p.Uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &p.PayloadBucket,
		Key:    &path,
		Body:   bytes.NewBuffer(marshaledEvent),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	msg := &apievents.AthenaS3EventPayload{
		Path:      path,
		VersionId: aws.ToString(out.VersionID),
	}
	buf, err := msg.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	base64Body := base64.StdEncoding.EncodeToString(buf)
	const s3BasedTrue = true
	return trace.Wrap(p.MessagePublisher.Publish(ctx, base64Body, s3BasedTrue))
}
