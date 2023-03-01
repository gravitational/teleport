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

package saml

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestInitIdP(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	// Standard https port should be stripped off.
	svcs := samlTestServiceWithURL(ctx, t, clock, "https://test.url:443")
	require.Equal(t, "test.url", svcs.samlIdP.idp.MetadataURL.Host)

	// A non-standard https port should still be present in the host.
	svcs = samlTestServiceWithURL(ctx, t, clock, "https://test.url:12345")
	require.Equal(t, "test.url:12345", svcs.samlIdP.idp.MetadataURL.Host)
}
