/*
Copyright 2015 Gravitational, Inc.

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

package tool

// Default port numbers used by all teleport tools
const (
	// Web UI over HTTP
	HTTPListenPort int16 = 3080

	// Web UI over HTTPS
	HTTPSecureListenPort int16 = 3081

	// When running in "SSH Server" mode behind a proxy, this
	// listening port will be used to connect users to:
	SSHServerListenPort int16 = 3022

	// When running as a "SSH Proxy" this port will be used to
	// accept connections and proxy them to SSHServerListenPort of
	// one of many SSH servers
	SSHProxyListenPort int16 = 3023

	// When running as a "SSH Proxy" this port will be used to
	// serve auth requests.
	AuthListenPort int16 = 3024
)

const (
	DefaultConfigFilePath = "/etc/teleport.yaml"

	// This is where all mutable data is stored (user keys, recorded sessions,
	// registered SSH servers, etc):
	DefaultDataDir = "/var/lib/teleport"

	// By default SSH server (and SSH proxy) will bind to this IP
	DefaultBindIP = "0.0.0.0"
)
