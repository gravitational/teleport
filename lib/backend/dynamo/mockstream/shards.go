package mockstream

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

type shard struct {
	id     string
	parent *string
	start  uint64
	end    uint64

	mu            sync.Mutex // Protects below
	children      []string
	records       []streamtypes.Record
	startSequence *string
	endSequence   *string
}

func (s *shard) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isClosedLocked()
}

func (s *shard) isClosedLocked() bool {
	return s.endSequence != nil
}

func (s *shard) closeLocked() {
	if s.endSequence != nil {
		return
	}

	if len(s.records) > 0 {
		last := s.records[len(s.records)-1]
		s.endSequence = last.Dynamodb.SequenceNumber
	}
}

func (s *shard) sequenceRange() *streamtypes.SequenceNumberRange {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.records) == 0 {
		return &streamtypes.SequenceNumberRange{
			StartingSequenceNumber: aws.String("0"),
			EndingSequenceNumber:   nil,
		}
	}

	return &streamtypes.SequenceNumberRange{
		StartingSequenceNumber: s.startSequence,
		EndingSequenceNumber:   s.endSequence,
	}

}

func (s *shard) append(r streamtypes.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.Dynamodb.SequenceNumber = aws.String(fmt.Sprintf("%d", len(s.records)))
	if len(s.records) == 0 {
		s.startSequence = r.Dynamodb.SequenceNumber
	}

	s.records = append(s.records, r)
}

func (s *shard) writtable(hash uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.isClosedLocked() && hash >= s.start && hash <= s.end
}

func (s *shard) split() ([]*shard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s == nil {
		return nil, trace.BadParameter("shard is nil!")
	}
	if len(s.children) > 0 {
		return nil, trace.BadParameter("shard %q already split", s.id)
	}

	if len(s.records) == 0 {
		return nil, trace.BadParameter("shard %q empty", s.id)
	}

	if s.start >= s.end {
		return nil, trace.BadParameter("shard %q too small to split", s.id)
	}

	mid := s.start + (s.end-s.start)/2

	// check the ranges are valid
	if mid < s.start || mid >= s.end {
		return nil, trace.BadParameter("invalid split for shard %q", s.id)
	}

	left := &shard{
		id:     newShardID(),
		parent: &s.id,
		start:  s.start,
		end:    mid,
	}

	right := &shard{
		id:     newShardID(),
		parent: &s.id,
		start:  mid + 1,
		end:    s.end,
	}

	s.children = []string{left.id, right.id}
	s.closeLocked()
	return []*shard{left, right}, nil
}

func (s *shard) spawn() (*shard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.children) > 0 {
		return nil, trace.BadParameter("shard %q already has children", s.id)
	}

	if len(s.records) == 0 {
		return nil, trace.BadParameter("shard %q empty", s.id)
	}

	child := &shard{
		id:     newShardID(),
		parent: &s.id,
		start:  s.start,
		end:    s.end,
	}

	s.children = []string{child.id}
	s.closeLocked()
	return child, nil
}

func (s *shard) toStreamType() streamtypes.Shard {
	return streamtypes.Shard{
		ShardId:             aws.String(s.id),
		ParentShardId:       s.parent,
		SequenceNumberRange: s.sequenceRange(),
	}
}

func (s *shard) getIter(itertype streamtypes.ShardIteratorType, seq string) (*string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var index int

	switch itertype {
	case streamtypes.ShardIteratorTypeTrimHorizon:
		index = 0
	case streamtypes.ShardIteratorTypeLatest:
		index = len(s.records)
	case streamtypes.ShardIteratorTypeAfterSequenceNumber:
		index = s.findIndexAfterSequence(seq)
	case streamtypes.ShardIteratorTypeAtSequenceNumber:
		index = s.findIndexAtSequence(seq)
	default:
		return nil, trace.BadParameter("unsupported iterator type %q", itertype)
	}

	return encodeIterator(iteratorState{
		ShardID: *aws.String(s.id),
		Index:   index,
	}), nil
}

func (s *shard) getRecords(start, limit int) (out []streamtypes.Record, next *string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = len(s.records)
	}

	end := min(start+limit, len(s.records))
	if start < len(s.records) {
		out = s.records[start:end]
	}

	if !s.isClosedLocked() || end < len(s.records) {
		next = encodeIterator(iteratorState{
			ShardID: s.id,
			Index:   end,
		})
	}

	return
}

func (s *shard) findIndexAfterSequence(seq string) int {
	for i, r := range s.records {
		if r.Dynamodb != nil && r.Dynamodb.SequenceNumber != nil {
			if *r.Dynamodb.SequenceNumber == seq {
				return i + 1
			}
		}
	}
	return len(s.records)
}

func (s *shard) findIndexAtSequence(seq string) int {
	for i, r := range s.records {
		if r.Dynamodb != nil && r.Dynamodb.SequenceNumber != nil {
			if *r.Dynamodb.SequenceNumber == seq {
				return i
			}
		}
	}
	return len(s.records)
}

func hashKey(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

func newShardID() string {
	return uuid.NewString()
}
