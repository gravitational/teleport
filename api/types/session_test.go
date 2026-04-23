// Copyright 2026 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package types

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAppSession_SetExpiry(t *testing.T) {
	// App session expiry is not handled by the backend because we need to emit audit
	// event before they are expired and deleted from the backend.  The SetExpiry method
	// from the Metadata is overwritten (to not set Metadata.Expires) to set expiry
	// in Spec.ResourceExpiry.

	certTTL := time.Now().Add(time.Hour).UTC()
	session, err := NewWebSession(uuid.NewString(), KindAppSession, WebSessionSpecV2{
		User:    "user1",
		Expires: certTTL,
	})
	require.NoError(t, err)

	sessV2 := session.(*WebSessionV2)

	// Metadata.Expires should be set, ResourceExpiry should be nil.
	require.NotNil(t, sessV2.Metadata.Expires)
	require.Nil(t, sessV2.Spec.ResourceExpiry)

	resourceTTL := time.Now().Add(time.Minute).UTC()
	session.SetExpiry(resourceTTL)

	require.NotNil(t, sessV2.Spec.ResourceExpiry)
	require.Equal(t, resourceTTL, *sessV2.Spec.ResourceExpiry)

	require.Equal(t, certTTL, *sessV2.Metadata.Expires)

	require.Equal(t, resourceTTL, session.Expiry())
}
