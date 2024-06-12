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
