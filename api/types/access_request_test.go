/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestAssertAccessRequestImplementsResourceWithLabels(t *testing.T) {
	ar, err := NewAccessRequest("test", "test", "test")
	require.NoError(t, err)
	require.Implements(t, (*ResourceWithLabels)(nil), ar)
}

func TestValidateAssumeStartTime(t *testing.T) {
	clock := clockwork.NewFakeClock()

	// Too far in the future.
	invalidStartTime := clock.Now().UTC().Add(1000000 * time.Hour)
	err := ValidateAssumeStartTime(invalidStartTime)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)

	// Valid Zero time.
	validTime := time.Time{}
	err = ValidateAssumeStartTime(validTime)
	require.Empty(t, err)

	// Valid time.
	validTime = clock.Now().UTC().Add(1 * time.Hour)
	err = ValidateAssumeStartTime(validTime)
	require.Empty(t, err)
}
