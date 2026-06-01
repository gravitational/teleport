/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
package dynamoevents

import (
	"context"
	"log/slog"
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

const (
	// chunkPrefix defines the chunk id prefix.
	chunkPrefix = "CHUNK#"
	// chunkStatusOpen defines the OPEN status.
	chunkStatusOpen = "OPEN"
	// chunkStatusClosed defines the CLOSED status.
	chunkStatusClosed = "CLOSED"

	// TODO: Tune chunk ttl and capcity with load tests.

	// chunkTTL defines the TTL of an OPEN chunk.
	chunkTTL = time.Minute * 3
	// chunkCapacity defines the maximum size of a chunk.
	chunkCapacity = 1000
)

// chunk identifies a chunk of events.
type chunk struct {
	// ID identifies a chunk.
	// The ID is written as SessionID in DynamoDB to satisfy the primary key
	// constraint. ID must always be set.
	ID string `dynamodbav:"SessionID"`
	// EventIndex is not used for chunks, but it is a required in DynamoDB to
	// satisfy the sort key constraint. EventIndex will always be 0.
	EventIndex int64
	// ChunkStatus indicates the status of the chunk.
	// This can be either "OPEN" or "CLOSED".
	ChunkStatus string
	// CreatedAt is the timestamp in unix format specifying when the chunk was
	// created. This is used to used to identify and close chunks that have passed
	// their OPEN status TTL. This is also used as a secondary DynamoDB index.
	CreatedAt int64 `json:",omitempty" dynamodbav:",omitempty"`
	// CreatedAtDate is used to identify the chunk partition.
	CreatedAtDate string `json:",omitempty" dynamodbav:",omitempty"`
}

// ChunkService is responsible for managing chunks.
type ChunkService struct {
	// dynamo is the dynamoDB API client.
	dynamo *dynamodb.Client
	// tableName is the dynamoDB chunk table name.
	tableName string
	// logger is emits log messages.
	logger *slog.Logger

	// chunkLock locks access to chunk status.
	chunkLock sync.Mutex
	// chunkID is the current open chunkID.
	chunkID string
	// chunkCount is the current number of events added to the open chunk.
	chunkCount int
	// chunkCreatedAt specifies the created at timestamp for the current open chunk.
	chunkCreatedAt time.Time

	// TODO: Figure out lease mechanism

	// chunkLeases keeps track of chunk leases.
	// A chunk lease must be acquired before writing the chunk, and it must
	// be released once done writing the chunk. This is required to ensure chunks
	// are closed after there are no more writers of a chunk.
	chunkLeases map[string]*chunkLease
	// staleChunks is the set of stale chunks that are pending closure.
	staleChunks map[string]struct{}
	// triggerReconcile triggers a reconcile to close stale chunks.
	triggerReconcile chan struct{}
}

// NewChunkService returns a new ChunkService.
func NewChunkService(
	dynamodbClient *dynamodb.Client,
	tableName string,
	logger *slog.Logger,
) *ChunkService {
	return &ChunkService{
		dynamo:           dynamodbClient,
		tableName:        tableName,
		logger:           logger,
		chunkLeases:      make(map[string]*chunkLease),
		staleChunks:      make(map[string]struct{}),
		triggerReconcile: make(chan struct{}, 1),
	}
}

// Run runs the ChunkService.
func (svc *ChunkService) Run(ctx context.Context) {
	svc.reconcileChunks(ctx)
}

// AcquireChunk acquires an open chunk lease.
func (svc *ChunkService) AcquireChunk(ctx context.Context) (string, error) {
	now := time.Now()

	svc.chunkLock.Lock()
	defer svc.chunkLock.Unlock()

	// Rotate if chunk is at capacity or ttl is expired.
	if svc.chunkID == "" ||
		svc.chunkCount >= chunkCapacity ||
		now.After(svc.chunkCreatedAt.Add(chunkTTL)) {

		if svc.chunkID != "" {
			// Defer chunk closure
			svc.staleChunks[svc.chunkID] = struct{}{}

			select {
			case svc.triggerReconcile <- struct{}{}:
			default:
			}

			svc.chunkID = ""
		}

		id, err := uuid.NewV7()
		if err != nil {
			return "", trace.Wrap(err, "failed to generate uuid")
		}

		chunkID := chunkPrefix + id.String()

		if err := svc.openChunk(ctx, chunkID, now); err != nil {
			return "", trace.Wrap(err, " failed to open chunk")
		}

		svc.chunkID = chunkID
		svc.chunkCreatedAt = now
		svc.chunkCount = 0
	}

	lease, ok := svc.chunkLeases[svc.chunkID]
	if !ok {
		lease = NewChunkLease()
		svc.chunkLeases[svc.chunkID] = lease
	}
	lease.Acquire()

	// Reserve capacity
	svc.chunkCount++

	return svc.chunkID, nil
}

// ReleaseChunk releases the chunk lease.
func (svc *ChunkService) ReleaseChunk(ctx context.Context, chunkID string) {
	svc.chunkLock.Lock()
	lease := svc.chunkLeases[chunkID]
	svc.chunkLock.Unlock()

	if lease == nil {
		svc.logger.DebugContext(ctx, "failed to release chunk; chunk lease not found",
			"chunk", chunkID)
		return
	}

	lease.Release()
}

func (svc *ChunkService) openChunk(ctx context.Context, chunkID string, createdAt time.Time) error {
	item, err := attributevalue.MarshalMap(chunk{
		ID:            chunkID,
		EventIndex:    0,
		CreatedAt:     createdAt.Unix(),
		CreatedAtDate: createdAt.Format(time.DateOnly),
		ChunkStatus:   chunkStatusOpen,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = svc.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(svc.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(SessionID)"),
	})

	return trace.Wrap(err)
}

func (svc *ChunkService) closeChunk(ctx context.Context, chunkID string) error {
	_, err := svc.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(svc.tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			keySessionID:  &dynamodbtypes.AttributeValueMemberS{Value: chunkID},
			keyEventIndex: &dynamodbtypes.AttributeValueMemberN{Value: "0"},
		},
		UpdateExpression:    aws.String("SET #status = :closed"),
		ConditionExpression: aws.String("#status = :open"),
		ExpressionAttributeNames: map[string]string{
			"#status": keyChunkStatus,
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":closed": &dynamodbtypes.AttributeValueMemberS{Value: chunkStatusClosed},
			":open":   &dynamodbtypes.AttributeValueMemberS{Value: chunkStatusOpen},
		},
	})

	if trace.IsAlreadyExists(convertError(err)) {
		svc.logger.DebugContext(ctx, "chunk is already closed", "chunk", chunkID)
		return nil
	}

	return trace.Wrap(err)
}

// reconcileChunks periodically attempts to close expired chunks.
func (svc *ChunkService) reconcileChunks(ctx context.Context) {
	const interval = time.Minute * 3

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			svc.logger.DebugContext(ctx, "reconcileChunks stopped")
			return
		case <-ticker.C:
			if err := svc.closeExpiredChunks(ctx); err != nil {
				svc.logger.WarnContext(ctx, "failed to close expired chunks", "error", err)
			}
		case <-svc.triggerReconcile:
			if err := svc.closeStaleChunks(ctx); err != nil {
				svc.logger.WarnContext(ctx, "failed to close stale chunks", "error", err)
			}
		}
	}
}

// closeExpiredChunks closes expired chunks that have been orphaned.
func (svc *ChunkService) closeExpiredChunks(ctx context.Context) error {
	expiredThreshold := time.Now().Add(-chunkTTL * 2).Unix()
	input := dynamodb.QueryInput{
		TableName:              aws.String(svc.tableName),
		IndexName:              aws.String(indexChunkStatusSearch),
		KeyConditionExpression: aws.String("#status = :status AND #createdAt <= :threshold"),
		ExpressionAttributeNames: map[string]string{
			"#status":    keyChunkStatus,
			"#createdAt": keyCreatedAt,
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":status":    &dynamodbtypes.AttributeValueMemberS{Value: chunkStatusOpen},
			":threshold": &dynamodbtypes.AttributeValueMemberN{Value: strconv.FormatInt(expiredThreshold, 10)},
		},
		// TODO: Handle pagination
		Limit: aws.Int32(1000),
	}

	start := time.Now()
	out, err := svc.dynamo.Query(ctx, &input)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: Remove debug logs
	svc.logger.DebugContext(ctx, "Successfully queried OPEN chunk IDs",
		"duration", time.Since(start),
		"items", len(out.Items),
	)

	for _, item := range out.Items {
		var result chunk
		if err := attributevalue.UnmarshalMap(item, &result); err != nil {
			return trace.Wrap(err, "failed to unmarshal chunk")
		}

		if err := svc.closeChunk(ctx, result.ID); err != nil {
			svc.logger.WarnContext(ctx, "failed to close chunk",
				"error", err,
				"chunk", result.ID)
			continue
		}

		// Clean up stale chunks in case callers never released chunk lease
		svc.chunkLock.Lock()
		delete(svc.chunkLeases, result.ID)
		delete(svc.staleChunks, result.ID)
		svc.chunkLock.Unlock()
	}
	return nil
}

// closeStaleChunks attempts to close stale chunks that are at capacity or have
// an expired ttl.
func (svc *ChunkService) closeStaleChunks(ctx context.Context) error {
	svc.chunkLock.Lock()
	staleChunks := maps.Keys(svc.staleChunks)
	svc.chunkLock.Unlock()

	for staleChunk := range staleChunks {
		svc.chunkLock.Lock()
		lease := svc.chunkLeases[staleChunk]
		svc.chunkLock.Unlock()

		// stale chunk has already been deleted
		if lease == nil {
			continue
		}

		if lease.Count() == 0 {
			// All leases have been released.
			if err := svc.closeChunk(ctx, staleChunk); err != nil {
				svc.logger.WarnContext(ctx, "failed to close chunk",
					"chunk", staleChunk,
					"error", err)
			}

			svc.chunkLock.Lock()
			delete(svc.chunkLeases, staleChunk)
			delete(svc.staleChunks, staleChunk)
			svc.chunkLock.Unlock()
		}
	}

	return nil
}

type chunkLease struct {
	mu    sync.Mutex
	count int
}

func NewChunkLease() *chunkLease {
	lease := &chunkLease{}
	return lease
}

func (l *chunkLease) Acquire() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count++
}

func (l *chunkLease) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.count > 0 {
		l.count--
	}
}

func (l *chunkLease) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}
