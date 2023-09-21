/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ai

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/ai/embedding"
)

// MockEmbedder returns embeddings based on the sha256 hash function. Those
// embeddings have no semantic meaning but ensure different embedded content
// provides different embeddings.
type MockEmbedder struct {
	mu          sync.Mutex
	TimesCalled map[string]int
}

func (m *MockEmbedder) ComputeEmbeddings(_ context.Context, input []string) ([]embedding.Vector64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]embedding.Vector64, len(input))
	for i, text := range input {
		name := strings.Split(text, "\n")[0]
		m.TimesCalled[name]++
		hash := sha256.Sum256([]byte(text))
		vector := make(embedding.Vector64, len(hash))
		for j, x := range hash {
			vector[j] = 1 / float64(int(x)+1)
		}
		result[i] = vector
	}
	return result, nil
}
