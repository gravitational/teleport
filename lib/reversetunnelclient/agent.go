// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reversetunnelclient

import "github.com/gravitational/teleport/api/constants"

const (
	// LocalNode is a special non-resolvable address that indicates the request
	// wants to connect to a dialed back node.
	LocalNode = "@local-node"
	// RemoteAuthServer is a special non-resolvable address that indicates client
	// requests a connection to the remote auth server.
	RemoteAuthServer = "@remote-auth-server"
	// LocalKubernetes is a special non-resolvable address that indicates that clients
	// requests a connection to the kubernetes endpoint of the local proxy.
	// This has to be a valid domain name, so it lacks @
	LocalKubernetes = "remote.kube.proxy." + constants.APIDomain
	// LocalWindowsDesktop is a special non-resolvable address that indicates
	// that clients requests a connection to the windows service endpoint of
	// the local proxy.
	// This has to be a valid domain name, so it lacks @
	LocalWindowsDesktop = "remote.windows_desktop.proxy." + constants.APIDomain
)
