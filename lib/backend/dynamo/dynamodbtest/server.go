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
	"net/http"
	"sync"

	"github.com/google/uuid"
)

// StreamARN is a constant ARN used for the mock stream.
var StreamARN = "arn:aws:dynamodb:us-east-1:123456789012:table/TableName/stream/2025-02-12T12:00:00.000"

// Server is a basic mock DynamoDB server.
type Server struct {
	mu           sync.RWMutex
	streamArn    string
	shardManager shardManager
	tableName    string
	Mux          *http.ServeMux
}

// NewMockDynamoDBServer creates a new mock DynamoDB server.
func NewMockDynamoDBServer() *Server {
	initialShardID := "shardId-" + uuid.NewString()
	m := &Server{
		tableName: "TableName",
		streamArn: StreamARN,
		shardManager: shardManager{
			shards: make(map[string]*Shard),
		},
	}
	m.shardManager.shards[initialShardID] = &Shard{
		ID:                     initialShardID,
		StartingSequenceNumber: 1,
		Records:                make([]Item, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleRequest)
	m.Mux = mux
	return m
}

type ShardState map[string]*Shard

// SetShardState allows tests to directly set the Shard manager state.
func (m *Server) SetShardState(shards ShardState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shardManager.shards = shards
}

// UpsertShard adds or replaces a single shard.
func (m *Server) UpsertShard(shard *Shard) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shardManager.shards[shard.ID] = shard
}

// AddRecord appends a stream record to an existing shard.
func (m *Server) AddRecord(shardID string, record Item) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shard := m.shardManager.shards[shardID]
	shard.Records = append(shard.Records, record)
}

// CloseShard marks a shard as closed, setting the ending sequence number to
// the last record's sequence number.
func (m *Server) CloseShard(shardID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shard := m.shardManager.shards[shardID]
	shard.IsClosed = true
	endingSeqNum := shard.getLastSequenceNumber()
	shard.EndingSequenceNum = &endingSeqNum
}
