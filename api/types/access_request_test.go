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

	"github.com/gravitational/teleport/api/constants"
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
	creation := clock.Now().UTC()
	day := 24 * time.Hour

	expiry := creation.Add(12 * day)
	maxAssumeStartDuration := creation.Add(constants.MaxAssumeStartDuration)

	// Start time too far in the future.
	invalidMaxedAssumeStartTime := creation.Add(constants.MaxAssumeStartDuration + (1 * day))
	err := ValidateAssumeStartTime(invalidMaxedAssumeStartTime, expiry, creation)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	require.ErrorIs(t, err, trace.BadParameter("assume start time is too far in the future, latest time allowed %q",
		maxAssumeStartDuration.Format(time.RFC3339)))

	// Expired start time.
	invalidExpiredAssumeStartTime := creation.Add(100 * day)
	err = ValidateAssumeStartTime(invalidExpiredAssumeStartTime, expiry, creation)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	require.ErrorIs(t, err, trace.BadParameter("assume start time cannot equal or exceed access expiry time at: %q",
		expiry.Format(time.RFC3339)))

	// Before creation start time.
	invalidBeforeCreationStartTime := creation.Add(-10 * day)
	err = ValidateAssumeStartTime(invalidBeforeCreationStartTime, expiry, creation)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	require.ErrorIs(t, err, trace.BadParameter("assume start time has to be greater than: %q",
		creation.Format(time.RFC3339)))

	// Valid start time.
	validStartTime := creation.Add(6 * day)
	err = ValidateAssumeStartTime(validStartTime, expiry, creation)
	require.NoError(t, err)
}
