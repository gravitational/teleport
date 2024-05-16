/*
Copyright 2015-2021 Gravitational, Inc.

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

package auth

import (
	"github.com/gravitational/roundtrip"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// MissingNamespaceError indicates that the client failed to
// provide the namespace in the request.
const MissingNamespaceError = authclient.MissingNamespaceError

type APIClient = authclient.Client
type Client = authclient.Client
type ClientI = authclient.ClientI

// NewClient creates a new API client with a connection to a Teleport server.
//
// The client will use the first credentials and the given dialer. If
// no dialer is given, the first address will be used. This address must
// be an auth server address.
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
func NewClient(cfg client.Config, params ...roundtrip.ClientParam) (*authclient.Client, error) {
	return authclient.NewClient(cfg, params...)
}
