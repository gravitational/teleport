// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decisionv1_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

func TestDecisionService_notImplemented(t *testing.T) {
	t.Parallel()

	env := NewTestenv(t)
	decisionClient := env.DecisionClient
	ctx := context.Background()

	t.Run("EvaluateSSHAccess", func(t *testing.T) {
		_, err := decisionClient.EvaluateSSHAccess(ctx, &decisionpb.EvaluateSSHAccessRequest{})
		assert.ErrorAs(t, err, new(*trace.NotImplementedError), "EvaluateSSHAccess error mismatch")
	})

	t.Run("EvaluateDatabaseAccess", func(t *testing.T) {
		_, err := decisionClient.EvaluateDatabaseAccess(ctx, &decisionpb.EvaluateDatabaseAccessRequest{})
		assert.ErrorAs(t, err, new(*trace.NotImplementedError), "EvaluateDatabaseAccess error mismatch")
	})
}
