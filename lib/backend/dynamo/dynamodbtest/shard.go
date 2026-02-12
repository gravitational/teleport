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

package dynamodbtest

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/google/uuid"
)

// CreateStreamRecord creates a DynamoDB stream record for testing.
func CreateStreamRecord(itemName string, seqNum int64) Item {
	return Item{
		"eventID":   uuid.NewString(),
		"eventName": "INSERT",
		"dynamodb": map[string]any{
			"Keys": map[string]any{
				"FullPath": map[string]any{
					"S": "teleport/test/" + itemName,
				},
			},
			"NewImage": map[string]any{
				"HashKey": map[string]any{"S": "teleport"},
				"FullPath": map[string]any{
					"S": "teleport/test/" + itemName,
				},
				"Value": map[string]any{
					"B": base64.StdEncoding.EncodeToString([]byte("test-value")),
				},
			},
			"SequenceNumber":              fmt.Sprintf("%d", seqNum),
			"StreamViewType":              "NEW_IMAGE",
			"ApproximateCreationDateTime": float64(time.Now().Unix()),
		},
	}
}

// Item represents a DynamoDB stream record
type Item map[string]any

// getSequenceNumber extracts the sequence number from a DynamoDB stream record.
// Returns 0 if the sequence number cannot be extracted.
func (item Item) getSequenceNumber() int64 {
	dynamodbField := item["dynamodb"]

	var dynamodb map[string]any
	switch v := dynamodbField.(type) {
	case Item:
		dynamodb = v
	case map[string]any:
		dynamodb = v
	default:
		return 0
	}

	if seqStr, ok := dynamodb["SequenceNumber"].(string); ok {
		seqNum, err := strconv.ParseInt(seqStr, 10, 64)
		if err != nil {
			panic(err)
		}
		return seqNum
	}

	return 0
}

// Shard represents a DynamoDB Stream Shard
type Shard struct {
	ID                     string
	Records                []Item
	StartingSequenceNumber int64
	ParentShardID          string
	EndingSequenceNum      *int64
	IsClosed               bool // When true, GetRecords will return nil NextShardIterator
}

// getLastSequenceNumber returns the sequence number of the last assigned record.
// Returns StartingSequenceNumber - 1 if no records have been assigned yet.
func (s *Shard) getLastSequenceNumber() int64 {
	if len(s.Records) == 0 {
		return s.StartingSequenceNumber - 1
	}

	if seqNum := s.Records[len(s.Records)-1].getSequenceNumber(); seqNum > 0 {
		return seqNum
	}

	return s.StartingSequenceNumber - 1
}

// shardManager manages DynamoDB Stream shards
type shardManager struct {
	shards map[string]*Shard
}

// encodeShardIterator creates an iterator string from iterator info
func encodeShardIterator(info shardIteratorInfo) string {
	return fmt.Sprintf("%s:%s:%d:%s", info.ShardID, info.IteratorType, info.SequenceNumber, uuid.NewString())
}

// decodeShardIterator parses an iterator string
func decodeShardIterator(iterator string) (shardIteratorInfo, error) {
	parts := strings.Split(iterator, ":")
	if len(parts) != 4 {
		return shardIteratorInfo{}, fmt.Errorf("invalid iterator format")
	}

	var parsedSeqNum int64
	_, err := fmt.Sscanf(parts[2], "%d", &parsedSeqNum)
	if err != nil && parts[1] != string(streamtypes.ShardIteratorTypeTrimHorizon) && parts[1] != string(streamtypes.ShardIteratorTypeLatest) {
		return shardIteratorInfo{}, fmt.Errorf("invalid sequence number in iterator")
	}

	return shardIteratorInfo{
		ShardID:        parts[0],
		IteratorType:   streamtypes.ShardIteratorType(parts[1]),
		SequenceNumber: parsedSeqNum,
	}, nil
}

// shardIteratorInfo encodes information about where to start reading from a Shard
type shardIteratorInfo struct {
	ShardID      string
	IteratorType streamtypes.ShardIteratorType
	// SequenceNumber is record serries number used for AT_SEQUENCE_NUMBER and AFTER_SEQUENCE_NUMBER
	SequenceNumber int64
}

// filterRecordsFromSequence filters Records based on sequence number
func (m *Server) filterRecordsFromSequence(records []Item, seqNum int64, inclusive bool) []Item {
	result := make([]Item, 0)
	for _, record := range records {
		recordSeq := record.getSequenceNumber()
		if recordSeq == 0 {
			continue
		}
		if inclusive && recordSeq >= seqNum {
			result = append(result, record)
		} else if !inclusive && recordSeq > seqNum {
			result = append(result, record)
		}
	}
	return result
}
