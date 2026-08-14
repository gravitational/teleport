package dsqlbk

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

func (b *Backend) runChangeFeed(ctx context.Context, awsConfig aws.Config) {
	for ctx.Err() == nil {
		err := b.runChangeFeedOnce(ctx, awsConfig)
		if ctx.Err() != nil {
			return
		}

		b.log.WarnContext(ctx, "Change feed failed, retrying", "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(b.cfg.StreamRetryInterval)):
		}
	}
}

func (b *Backend) runChangeFeedOnce(ctx context.Context, awsConfig aws.Config) error {
	clt := kinesis.NewFromConfig(awsConfig)

	shardIDs, err := listOpenShardIDs(ctx, clt, b.cfg.KinesisStreamName)
	if err != nil {
		if ctx.Err() != nil {
			return trace.Wrap(ctx.Err())
		}
		return trace.Wrap(err)
	}

	initBarrier := make(chan struct{}, len(shardIDs))
	initDone := make(chan struct{})

	filter, err := newEventOrderFilter(time.Duration(b.cfg.StreamReorderGraceInterval))
	if err != nil {
		return trace.Wrap(err)
	}

	eg, ctx := errgroup.WithContext(ctx)

	var followShard func(shardID string, startingSequenceNumber string) error
	followShard = func(shardID string, startingSequenceNumber string) (retErr error) {
		req := &kinesis.GetShardIteratorInput{
			ShardId:           &shardID,
			ShardIteratorType: "LATEST",
			StreamName:        &b.cfg.KinesisStreamName,
		}
		init := startingSequenceNumber == ""
		if !init {
			req.ShardIteratorType = "AT_SEQUENCE_NUMBER"
			req.StartingSequenceNumber = &startingSequenceNumber
		}

		b.log.DebugContext(ctx,
			"Fetching events from shard",
			"shard_id", shardID,
			"init", init,
		)
		defer func() {
			b.log.DebugContext(ctx,
				"Done fetching events from shard",
				"shard_id", shardID,
				"error", retErr)
		}()

		nextEmitLagWarnNanos := time.Now()
		nextReadLagWarn := time.Now()

		ticker := time.NewTicker(time.Duration(b.cfg.StreamPollInterval))
		defer ticker.Stop()

		// wait for the first tick to help guarantee that we won't hit a rate
		// limit on GetShardIterator or GetRecords - this shouldn't really
		// happen outside of tests that close and reopen backends pointed at the
		// same DSQL cluster, but technically the stream retry interval could be
		// configured to be low enough to hit the 5 rps rate limit
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.C:
		}

		resp, err := clt.GetShardIterator(ctx, req)
		if err != nil {
			if ctx.Err() != nil {
				return trace.Wrap(ctx.Err())
			}
			return trace.Wrap(err)
		}

		shardIterator := resp.ShardIterator
		for {
			resp, err := clt.GetRecords(ctx, &kinesis.GetRecordsInput{
				ShardIterator: shardIterator,
			})
			if err != nil {
				if ctx.Err() != nil {
					return trace.Wrap(ctx.Err())
				}
				return trace.Wrap(err)
			}

			if init {
				init = false
				initBarrier <- struct{}{}
				select {
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				case <-initDone:
				}
			}

			receiveLag := time.Duration(aws.ToInt64(resp.MillisBehindLatest)) * time.Millisecond
			if receiveLag > 10*time.Second && time.Now().After(nextReadLagWarn) {
				b.log.WarnContext(ctx,
					"Change feed receive latency is higher than 10 seconds, if this condition persists it might be caused by a misconfiguration of the Kinesis data stream or by too many Auth Service instances using the same Kinesis data stream.",
					"lag", receiveLag.String(),
					"shard_id", shardID,
				)
				nextReadLagWarn = time.Now().Add(30 * time.Second)
			}
			if len(resp.Records) > 0 && !b.cfg.DisableMetrics {
				eventReceiveLatencySecondsHistogram.Observe(receiveLag.Seconds())
			}

			events := make([]cdcEvent, 0, len(resp.Records))
			for _, r := range resp.Records {
				event, err := parseRecord(r.Data, b.cfg.DSQLIdentifier, b.cfg.DatabaseName, b.cfg.DatabaseSchema)
				if err != nil {
					return trace.Wrap(err, "parsing cdc records")
				}
				if event.event != nil {
					emitLag := event.emitTimestamp - event.commitTimestamp
					if !b.cfg.DisableMetrics {
						eventEmitLatencySecondsHistogram.Observe(emitLag.Seconds())
					}
					if emitLag > 10*time.Second && time.Now().After(nextEmitLagWarnNanos) {
						b.log.WarnContext(ctx,
							"Change feed emit latency is higher than 10 seconds, potentially as a result of Kinesis autoscaling. If this condition persists it might be caused by a misconfiguration of the DSQL change feed stream.",
							"lag", emitLag.String(),
							"shard_id", shardID,
						)
						nextEmitLagWarnNanos = time.Now().Add(30 * time.Second)
					}

					events = append(events, event)
				}
			}

			if err := filter.emitEvents(events, b.buf); err != nil {
				return trace.Wrap(err)
			}

			if resp.NextShardIterator != nil {
				shardIterator = resp.NextShardIterator

				select {
				case <-ticker.C:
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				}

				continue
			}

			if len(resp.ChildShards) < 1 {
				b.log.ErrorContext(ctx,
					"Got no child shards at shard end (this is a bug)",
					"shard_id", shardID,
				)
				return trace.BadParameter("got no child shards at shard end (this is a bug)")
			}
			if len(resp.ChildShards) > 2 {
				childShardIDs := make([]string, 0, len(resp.ChildShards))
				for _, child := range resp.ChildShards {
					childShardIDs = append(childShardIDs, *child.ShardId)
				}
				b.log.WarnContext(ctx,
					"Got more than two child shards at shard end, which is unexpected",
					"shard_id", shardID,
					"child_shard_ids", childShardIDs,
				)
			}
			for _, child := range resp.ChildShards {
				switch len(child.ParentShards) {
				case 0:
					b.log.ErrorContext(ctx,
						"Got child shard with no parents (this is a bug)",
						"shard_id", shardID,
						"child_shard_id", *child.ShardId,
					)
					return trace.BadParameter("got child shard with no parents (this is a bug)")
				case 1:
				case 2:
					// supporting shard merges requires synchronizing the end of
					// the two parent shards, and in on-demand mode it should be
					// quite rare for the stream to scale down and for shards to
					// merge, so for now we just ignore the problem and reset
					// the stream
					b.log.InfoContext(ctx,
						"Got child shard two parents as a result of Kinesis downscaling, currently not supported",
						"shard_id", shardID,
						"child_shard_id", *child.ShardId,
						"parent_shards", child.ParentShards,
					)
					return trace.BadParameter("got child shard with two parents, resetting stream (currently unsupported)")
				default:
					b.log.ErrorContext(ctx,
						"Got child shard with more than two parents (this is a bug)",
						"shard_id", shardID,
						"child_shard_id", *child.ShardId,
						"parent_shards", child.ParentShards,
					)
					return trace.BadParameter("got child shard with more than two parents (this is a bug)")
				}
			}
			// the only way to get the StartingSequenceNumber for the one or two
			// child shards we have is seemingly a full unfiltered list,
			// unfortunately; ListShards has incredibly generous rate limits
			// however, so it should be fine
			childShards, err := listChildShards(ctx, clt, b.cfg.KinesisStreamName, resp.ChildShards)
			if err != nil {
				if ctx.Err() != nil {
					return trace.Wrap(ctx.Err())
				}
				b.log.WarnContext(ctx, "Failed to list shards after shard end", "shard_id", shardID, "error", err)
				return trace.Wrap(err, "listing shards after shard end")
			}
			for _, childShard := range childShards {
				childShardID := *childShard.ShardId
				startingSequenceNumber := *childShard.SequenceNumberRange.StartingSequenceNumber
				if startingSequenceNumber == "" {
					// we use an empty StartingSequenceNumber to signify that
					// the shard is part of the first shards and should wait for
					// init, so if we let a blank string through we could end up
					// with a goroutine blocked forever
					b.log.WarnContext(ctx, "Got an empty StartingSequenceNumber from ListShards (this is a bug)", "child_shard_id", childShardID, "shard_id", shardID)
					return trace.BadParameter("got an empty StartingSequenceNumber from ListShards (this is a bug)")
				}
				b.log.InfoContext(ctx, "Fetching events from child shard", "child_shard_id", childShardID, "parent_shard_id", shardID)
				eg.Go(func() error {
					return followShard(childShardID, startingSequenceNumber)
				})
			}
			return nil
		}
	}

	b.log.InfoContext(ctx, "Fetching events from open shards", "shard_ids", shardIDs)
	for _, shardID := range shardIDs {
		eg.Go(func() error {
			return followShard(shardID, "")
		})
	}

	for range shardIDs {
		select {
		case <-initBarrier:
		case <-ctx.Done():
			return eg.Wait()
		}
	}

	b.buf.SetInit()
	defer b.buf.Reset()
	b.log.InfoContext(ctx, "Event stream initialized")
	defer b.log.WarnContext(ctx, "Event stream done")
	close(initBarrier)
	close(initDone)

	return eg.Wait()
}

type cdcKVBefore struct {
	Key string `json:"key"`
}

type cdcKVAfter struct {
	Key string `json:"key"`

	Value    []byte `json:"value"`
	Expiry   int64  `json:"expiry"`
	Revision string `json:"revision"`
}

type cdcRecord struct {
	Type string `json:"type"`
	Op   string `json:"op"`

	Before sharedRawMessage `json:"before"`
	After  sharedRawMessage `json:"after"`

	Source struct {
		Version string `json:"version"`

		Cluster  string `json:"cluster"`
		Database string `json:"db"`
		Schema   string `json:"schema"`
		Table    string `json:"table"`

		CommitTimestamp int64 `json:"ts_ns"`
	} `json:"source"`

	EmitTimestamp int64 `json:"ts_ns"`
}

type cdcEvent struct {
	event           *backend.Event
	commitTimestamp time.Duration
	emitTimestamp   time.Duration
}

func parseRecord(data []byte, clusterIdentifier, databaseName, schema string) (cdcEvent, error) {
	var r cdcRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return cdcEvent{}, trace.Wrap(err)
	}

	if r.Type == "fragment" {
		return cdcEvent{}, nil
	}

	if r.Source.Version != "1.0" {
		return cdcEvent{}, trace.BadParameter("expected CDC version 1.0, got %+q", r.Source.Version)
	}

	if r.Source.Cluster != clusterIdentifier || r.Source.Database != databaseName || r.Source.Schema != schema || r.Source.Table != "kv" {
		return cdcEvent{}, nil
	}

	if r.Type != "full" {
		return cdcEvent{}, trace.BadParameter("got event with unsupported type %+q, expected full", r.Type)
	}

	if r.Source.CommitTimestamp == 0 || r.EmitTimestamp == 0 {
		return cdcEvent{}, trace.BadParameter("got event with missing timestamps")
	}

	switch r.Op {
	case "c", "u":
		if len(r.After) < 1 {
			return cdcEvent{}, trace.BadParameter("got kv event with op %+q and null after", r.Op)
		}
		var after *cdcKVAfter
		if err := json.Unmarshal(r.After, &after); err != nil {
			return cdcEvent{}, trace.Wrap(err, "unmarshaling after for kv event with op %+q", r.Op)
		}
		if after == nil {
			return cdcEvent{}, trace.BadParameter("got kv event with op %+q and null after", r.Op)
		}

		if after.Expiry <= -9223372036832400000 {
			// an item with -infinity expiry can be a tombstone used as part of
			// a nonexistence condition in AtomicWrite; if this is a create we
			// can skip the event altogether, but if it's an update (which
			// shouldn't really happen) we have to emit an event for it, but we
			// can render it as an OpDelete event because the item is guaranteed
			// to not be existing from the point of view of item expiry

			if r.Op == "c" {
				return cdcEvent{}, nil
			}

			return cdcEvent{
				event: &backend.Event{
					Type: apitypes.OpDelete,
					Item: backend.Item{
						Key: backend.KeyFromString(after.Key),
					},
				},
				commitTimestamp: time.Duration(r.Source.CommitTimestamp) * time.Nanosecond,
				emitTimestamp:   time.Duration(r.EmitTimestamp) * time.Nanosecond,
			}, nil
		}

		e := &backend.Event{
			Type: apitypes.OpPut,
			Item: backend.Item{
				Key:      backend.KeyFromString(after.Key),
				Value:    after.Value,
				Revision: after.Revision,
			},
		}
		if after.Expiry < 9223372036825200000 {
			e.Item.Expires = time.UnixMicro(after.Expiry)
		}

		return cdcEvent{
			event:           e,
			commitTimestamp: time.Duration(r.Source.CommitTimestamp) * time.Nanosecond,
			emitTimestamp:   time.Duration(r.EmitTimestamp) * time.Nanosecond,
		}, nil

	case "d":
		if len(r.Before) < 1 {
			return cdcEvent{}, trace.BadParameter("got kv event with op \"d\" and null before")
		}
		var before *cdcKVBefore
		if err := json.Unmarshal(r.Before, &before); err != nil {
			return cdcEvent{}, trace.Wrap(err, "unmarshaling before for kv event with op \"d\"")
		}
		if before == nil {
			return cdcEvent{}, trace.BadParameter("got kv event with op \"d\" and null before")
		}

		return cdcEvent{
			event: &backend.Event{
				Type: apitypes.OpDelete,
				Item: backend.Item{
					Key: backend.KeyFromString(before.Key),
				},
			},
			commitTimestamp: time.Duration(r.Source.CommitTimestamp) * time.Nanosecond,
			emitTimestamp:   time.Duration(r.EmitTimestamp) * time.Nanosecond,
		}, nil

	default:
		return cdcEvent{}, trace.BadParameter("got event with unsupported op %+q", r.Op)
	}
}

type shard struct {
	shardID         string
	startingHashKey *big.Int
	endingHashKey   *big.Int
}

// listOpenShardIDs returns a list of IDs of open shards for the given stream
// that have been checked to cover the full hash key range.
func listOpenShardIDs(ctx context.Context, clt *kinesis.Client, streamName string) ([]string, error) {
	req := &kinesis.ListShardsInput{
		ShardFilter: &kinesistypes.ShardFilter{
			Type: "AT_LATEST",
		},
		StreamName: &streamName,
	}

	var shards []shard
	for {
		resp, err := clt.ListShards(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, s := range resp.Shards {
			if s.SequenceNumberRange.EndingSequenceNumber != nil {
				// even with the AT_LATEST filter it's possible to get shards
				// that were very recently closed
				continue
			}

			starting, ok := new(big.Int).SetString(*s.HashKeyRange.StartingHashKey, 10)
			if !ok {
				return nil, trace.BadParameter("got malformed StartingHashKey in ListShards response (this is a bug)")
			}
			ending, ok := new(big.Int).SetString(*s.HashKeyRange.EndingHashKey, 10)
			if !ok {
				return nil, trace.BadParameter("got malformed EndingHashKey in ListShards response (this is a bug)")
			}
			if starting.Cmp(ending) > 0 {
				return nil, trace.BadParameter("got StartingHashKey greater than EndingHashKey in ListShards response (this is a bug)")
			}

			shards = append(shards, shard{
				shardID:         *s.ShardId,
				startingHashKey: starting,
				endingHashKey:   ending,
			})
		}
		if resp.NextToken == nil {
			break
		}
		req = &kinesis.ListShardsInput{
			NextToken: resp.NextToken,
		}
	}

	if len(shards) < 1 {
		return nil, trace.BadParameter("got no open shards (this is a bug)")
	}

	if err := checkShardCoverage(shards); err != nil {
		return nil, trace.Wrap(err)
	}

	shardIDs := make([]string, 0, len(shards))
	for _, s := range shards {
		shardIDs = append(shardIDs, s.shardID)
	}
	return shardIDs, nil
}

// checkShardCoverage checks that the list of shards covers the full hash key
// range from 0 to 2^128-1 with no overlaps or gaps, by sorting the list by
// start hash, then checking that the first shard starts from 0, the last ends
// at 2^128-1, and that beginning and end of adjacent shards differ by exactly 1.
func checkShardCoverage(shards []shard) error {
	slices.SortFunc(shards, func(a, b shard) int { return a.startingHashKey.Cmp(b.startingHashKey) })
	if c := shards[0].startingHashKey.Cmp(big.NewInt(0)); c > 0 {
		// startingHashKey[0] > 0
		return trace.BadParameter(
			"got open shards that don't cover the full hash key range (missing below %v, this is a bug)",
			shards[0].startingHashKey.String(),
		)
	} else if c < 0 {
		// startingHashKey[0] < 0
		return trace.BadParameter(
			"got open shards with unexpected hash key range coverage below 0 (down to %v, this is a bug)",
			shards[0].startingHashKey.String(),
		)
	}
	for i := range len(shards) - 1 {
		if c := new(big.Int).Sub(shards[i+1].startingHashKey, shards[i].endingHashKey).Cmp(big.NewInt(1)); c > 0 {
			// startingHashKey[i+1]-endingHashKey[i] > 1
			return trace.BadParameter(
				"got open shards that don't cover the full hash key range (missing between %v and %v, this is a bug)",
				shards[i].endingHashKey.String(), shards[i+1].startingHashKey.String(),
			)
		} else if c < 0 {
			// startingHashKey[i+1]-endingHashKey[i] < 1
			return trace.BadParameter(
				"got open shards with some overlap in hash key range (overlapping between %v and %v, this is a bug)",
				shards[i+1].startingHashKey.String(), shards[i].endingHashKey.String(),
			)
		}
	}

	// maxUint128 = (1 << 128) - 1
	maxUint128 := big.NewInt(1)
	maxUint128 = maxUint128.Lsh(maxUint128, 128)
	maxUint128 = maxUint128.Sub(maxUint128, big.NewInt(1))

	if c := shards[len(shards)-1].endingHashKey.Cmp(maxUint128); c < 0 {
		// endingHashKey[-1] < 2^128-1
		return trace.BadParameter(
			"got open shards that don't cover the full hash key range (missing above %v, this is a bug)",
			shards[len(shards)-1].endingHashKey.String(),
		)
	} else if c > 0 {
		// endingHashKey[-1] > 2^128-1
		return trace.BadParameter(
			"got open shards with unexpected hash key range coverage above 2^128-1 (up to %v, this is a bug)",
			shards[len(shards)-1].endingHashKey.String(),
		)
	}

	return nil
}

func listChildShards(ctx context.Context, clt *kinesis.Client, streamName string, childShards []kinesistypes.ChildShard) ([]*kinesistypes.Shard, error) {
	if len(childShards) < 1 {
		return nil, nil
	}
	out := make([]*kinesistypes.Shard, len(childShards))
	req := &kinesis.ListShardsInput{
		StreamName: &streamName,
	}
	for {
		resp, err := clt.ListShards(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for i := range resp.Shards {
			shard := &resp.Shards[i]
			for j := range childShards {
				if *shard.ShardId == *childShards[j].ShardId {
					out[j] = shard
				}
			}
		}
		if !slices.Contains(out, nil) {
			return out, nil
		}
		if resp.NextToken == nil {
			return nil, trace.BadParameter("child shard not found after parent shard finished")
		}
		req = &kinesis.ListShardsInput{
			NextToken: resp.NextToken,
		}
	}
}

// eventOrderFilter keeps track of the latest event for every key that we've
// seen, to skip events from the past that have arrived out of order (and that
// should not be sent through the fanout). To avoid keeping track of an
// unbounded amount of rows, periodically the storage is cleaned up by deleting
// all the keys older than some threshold, chosen to be some generous time
// before the latest event we've ever seen; if then an event arrives from before
// the threshold, since we can't know if the event is out of order or not, we
// return an error to reset the event stream.
type eventOrderFilter struct {
	graceInterval time.Duration

	mu sync.Mutex

	// invariants:
	// - maxTimestamp = max(events[*])
	// - timestampWatermark <= min(events[*])
	// - len(events) < nextCleanupSize

	events       map[string]time.Duration
	maxTimestamp time.Duration

	timestampWatermark time.Duration
	nextCleanupSize    int
}

func newEventOrderFilter(graceInterval time.Duration) (*eventOrderFilter, error) {
	if graceInterval <= 0 {
		return nil, trace.BadParameter("invalid stream_reorder_grace_interval (this is a bug)")
	}

	return &eventOrderFilter{
		graceInterval: graceInterval,

		events:       make(map[string]time.Duration, 1024),
		maxTimestamp: math.MinInt64,

		timestampWatermark: math.MinInt64,
		nextCleanupSize:    1024,
	}, nil
}

type eventEmitter interface {
	Emit(events ...backend.Event) (ok bool)
}

var _ eventEmitter = (*backend.CircularBuffer)(nil)

func (m *eventOrderFilter) emitEvents(events []cdcEvent, emitter eventEmitter) error {
	if len(events) < 1 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range events {
		// we have not deleted past events with a timestamp equal to
		// timestampWatermark so it's fine if we get an event at precisely
		// timestampWatermark
		if e.commitTimestamp < m.timestampWatermark {
			return trace.BadParameter("received event from before the current watermark")
		}
	}

	eventsToEmit := make([]backend.Event, 0, len(events))
	for _, e := range events {
		if t, ok := m.events[e.event.Item.Key.String()]; ok && t >= e.commitTimestamp {
			// the equal case can happen as a result of an event emitted more
			// than once
			continue
		}
		m.events[e.event.Item.Key.String()] = e.commitTimestamp
		m.maxTimestamp = max(m.maxTimestamp, e.commitTimestamp)
		eventsToEmit = append(eventsToEmit, *e.event)
	}

	emitter.Emit(eventsToEmit...)

	if len(m.events) >= m.nextCleanupSize {
		m.timestampWatermark = m.maxTimestamp - m.graceInterval
		for k, v := range m.events {
			if v < m.timestampWatermark {
				delete(m.events, k)
			}
		}
		// classic amortization strategy, we only iterate over the map every
		// time it doubles in size
		m.nextCleanupSize = max(1024, len(m.events)*2)
	}
	return nil
}
