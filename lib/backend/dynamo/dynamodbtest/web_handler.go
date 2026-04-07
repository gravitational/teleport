/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
)

const (
	tabelAPIPrefix   = "DynamoDB_20120810."
	streamsAPIPrefix = "DynamoDBStreams_20120810."
)

// handleRequest routes requests to the appropriate handler based on the X-Amz-Target header.
func (m *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")

	switch {
	case strings.HasPrefix(target, tabelAPIPrefix):
		m.handleDynamoDBRequest(w, r, strings.TrimPrefix(target, tabelAPIPrefix))
	case strings.HasPrefix(target, streamsAPIPrefix):
		m.handleStreamsRequest(w, r, strings.TrimPrefix(target, streamsAPIPrefix))
	default:
		http.Error(w, "Unknown target", http.StatusBadRequest)
	}
}

// handleDynamoDBRequest handles DynamoDB API requests.
func (m *Server) handleDynamoDBRequest(w http.ResponseWriter, r *http.Request, operation string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch operation {
	case "DescribeTable":
		m.handleDescribeTable(w, body)
	case "CreateTable":
		m.handleCreateTable(w, body)
	case "DescribeTimeToLive":
		m.handleDescribeTimeToLive(w, body)
	default:
		http.Error(w, "Unsupported operation", http.StatusNotImplemented)
	}
}

// handleStreamsRequest handles DynamoDB Streams API requests.
func (m *Server) handleStreamsRequest(w http.ResponseWriter, r *http.Request, operation string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch operation {
	case "DescribeStream":
		m.handleDescribeStream(w, body)
	case "GetShardIterator":
		m.handleGetShardIterator(w, body)
	case "GetRecords":
		m.handleGetRecords(w, body)
	default:
		http.Error(w, "Unsupported operation", http.StatusNotImplemented)
	}
}

// handleDescribeTable handles DescribeTable requests.
func (m *Server) handleDescribeTable(w http.ResponseWriter, body []byte) {
	m.writeResponse(w, map[string]any{
		"Table": map[string]any{
			"TableName":       m.tableName,
			"TableStatus":     "ACTIVE",
			"LatestStreamArn": m.streamArn,
			"AttributeDefinitions": []map[string]string{
				{"AttributeName": "HashKey", "AttributeType": "S"},
				{"AttributeName": "FullPath", "AttributeType": "S"},
			},
			"StreamSpecification": map[string]any{
				"StreamEnabled":  true,
				"StreamViewType": "NEW_IMAGE",
			},
		},
	})
}

// handleCreateTable handles CreateTable requests.
func (m *Server) handleCreateTable(w http.ResponseWriter, body []byte) {
	m.writeResponse(w, map[string]any{
		"TableDescription": map[string]any{
			"TableName":   m.tableName,
			"TableStatus": "ACTIVE",
		},
	})
}

func (m *Server) handleDescribeTimeToLive(w http.ResponseWriter, body []byte) {
	m.writeResponse(w, map[string]any{
		"TimeToLiveDescription": map[string]any{
			"TimeToLiveStatus": "ENABLED",
			"AttributeName":    "Expires",
		},
	})
}

func (m *Server) handleDescribeStream(w http.ResponseWriter, body []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build list of all shards (both active and closed)
	shards := make([]map[string]any, 0, len(m.shardManager.shards))
	for shardID, shard := range m.shardManager.shards {
		shardInfo := map[string]any{
			"ShardId": shardID,
		}

		// Add parent Shard ID if this is a child Shard
		if shard.ParentShardID != "" {
			shardInfo["ParentShardId"] = shard.ParentShardID
		}

		seqRange := map[string]any{}
		if shard.EndingSequenceNum != nil {
			seqRange["EndingSequenceNumber"] = fmt.Sprintf("%d", *shard.EndingSequenceNum)
		}
		shardInfo["SequenceNumberRange"] = seqRange

		shards = append(shards, shardInfo)
	}

	m.writeResponse(w, map[string]any{
		"StreamDescription": map[string]any{
			"StreamArn":    m.streamArn,
			"StreamStatus": "ENABLED",
			"Shards":       shards,
		},
	})
}

func (m *Server) writeError(w http.ResponseWriter, errorType, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusBadRequest)
	resp := map[string]string{
		"__type":  errorType,
		"message": message,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}
}

func (m *Server) writeResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func (m *Server) handleGetShardIterator(w http.ResponseWriter, body []byte) {
	var req struct {
		ShardID           string  `json:"ShardId"`
		ShardIteratorType string  `json:"ShardIteratorType"`
		SequenceNumber    *string `json:"SequenceNumber,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		m.writeError(w, "ValidationException", err.Error())
		return
	}

	m.mu.RLock()
	shard, exists := m.shardManager.shards[req.ShardID]
	m.mu.RUnlock()

	if !exists {
		m.writeError(w, "ResourceNotFoundException", fmt.Sprintf("Shard %s not found", req.ShardID))
		return
	}

	// Parse sequence number if provided
	var seqNum int64
	if req.SequenceNumber != nil {
		fmt.Sscanf(*req.SequenceNumber, "%d", &seqNum)
	}

	// For LATEST, position after all existing records
	// Only records added AFTER the iterator is created will be returned
	if req.ShardIteratorType == string(streamtypes.ShardIteratorTypeLatest) {
		m.mu.RLock()
		seqNum = shard.getLastSequenceNumber()
		m.mu.RUnlock()
	}

	iteratorInfo := shardIteratorInfo{
		ShardID:        req.ShardID,
		IteratorType:   streamtypes.ShardIteratorType(req.ShardIteratorType),
		SequenceNumber: seqNum,
	}

	iterator := encodeShardIterator(iteratorInfo)

	m.writeResponse(w, map[string]any{
		"ShardIterator": iterator,
	})
}

func (m *Server) handleGetRecords(w http.ResponseWriter, body []byte) {
	var req struct {
		ShardIterator string `json:"ShardIterator"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		m.writeError(w, "ValidationException", err.Error())
		return
	}

	// Parse iterator
	iteratorInfo, err := decodeShardIterator(req.ShardIterator)
	if err != nil {
		m.writeError(w, "ValidationException", "Invalid Shard iterator format")
		return
	}

	m.mu.Lock()
	shard, exists := m.shardManager.shards[iteratorInfo.ShardID]
	if !exists {
		m.mu.Unlock()
		m.writeError(w, "ResourceNotFoundException", fmt.Sprintf("Shard %s not found", iteratorInfo.ShardID))
		return
	}

	// Filter Records based on iterator type
	var recordsToReturn []Item

	switch iteratorInfo.IteratorType {
	case streamtypes.ShardIteratorTypeTrimHorizon:
		recordsToReturn = make([]Item, len(shard.Records))
		copy(recordsToReturn, shard.Records)

	case streamtypes.ShardIteratorTypeLatest:
		recordsToReturn = m.filterRecordsFromSequence(shard.Records, iteratorInfo.SequenceNumber, false)

	case streamtypes.ShardIteratorTypeAtSequenceNumber:
		recordsToReturn = m.filterRecordsFromSequence(shard.Records, iteratorInfo.SequenceNumber, true)

	case streamtypes.ShardIteratorTypeAfterSequenceNumber:
		recordsToReturn = m.filterRecordsFromSequence(shard.Records, iteratorInfo.SequenceNumber, false)
	default:
		panic("iterator type is required")
	}

	newSeqNum := iteratorInfo.SequenceNumber
	if len(recordsToReturn) > 0 {
		if lastRecord := recordsToReturn[len(recordsToReturn)-1]; lastRecord != nil {
			if seqNum := lastRecord.getSequenceNumber(); seqNum > 0 {
				newSeqNum = seqNum
			}
		}
	} else if iteratorInfo.IteratorType == streamtypes.ShardIteratorTypeLatest {
		// For LATEST with no Records, update to last sequence
		newSeqNum = shard.getLastSequenceNumber()
	}

	m.mu.Unlock()

	// Generate next iterator - continue from where we left off
	// TRIM_HORIZON and LATEST only apply to the initial iterator position.
	// NextShardIterator should continue sequentially from the last returned record.
	nextIteratorType := iteratorInfo.IteratorType
	if iteratorInfo.IteratorType == streamtypes.ShardIteratorTypeTrimHorizon || iteratorInfo.IteratorType == streamtypes.ShardIteratorTypeLatest {
		nextIteratorType = streamtypes.ShardIteratorTypeAfterSequenceNumber
	}

	response := map[string]any{
		"Records": recordsToReturn,
	}

	// Only include NextShardIterator if the shard is not closed
	if !shard.IsClosed {
		nextIteratorInfo := shardIteratorInfo{
			ShardID:        iteratorInfo.ShardID,
			IteratorType:   nextIteratorType,
			SequenceNumber: newSeqNum,
		}
		nextIterator := encodeShardIterator(nextIteratorInfo)
		response["NextShardIterator"] = nextIterator
	}

	m.writeResponse(w, response)
}
