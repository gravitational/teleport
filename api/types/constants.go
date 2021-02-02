/*
Copyright 2020-2021 Gravitational, Inc.

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

const (
	// DefaultAPIGroup is a default group of permissions API,
	// lets us to add different permission types
	DefaultAPIGroup = "gravitational.io/teleport"

	// ActionRead grants read access (get, list)
	ActionRead = "read"

	// ActionWrite allows to write (create, update, delete)
	ActionWrite = "write"

	// Wildcard is a special wildcard character matching everything
	Wildcard = "*"

	// KindNamespace is a namespace
	KindNamespace = "namespace"

	// KindUser is a user resource
	KindUser = "user"

	// KindKeyPair is a public/private key pair
	KindKeyPair = "key_pair"

	// KindHostCert is a host certificate
	KindHostCert = "host_cert"

	// KindJWT is a JWT token signer.
	KindJWT = "jwt"

	// KindLicense is a license resource
	KindLicense = "license"

	// KindRole is a role resource
	KindRole = "role"

	// KindAccessRequest is an AccessRequest resource
	KindAccessRequest = "access_request"

	// KindPluginData is a PluginData resource
	KindPluginData = "plugin_data"

	// KindOIDC is OIDC connector resource
	KindOIDC = "oidc"

	// KindSAML is SAML connector resource
	KindSAML = "saml"

	// KindGithub is Github connector resource
	KindGithub = "github"

	// KindOIDCRequest is OIDC auth request resource
	KindOIDCRequest = "oidc_request"

	// KindSAMLRequest is SAML auth request resource
	KindSAMLRequest = "saml_request"

	// KindGithubRequest is Github auth request resource
	KindGithubRequest = "github_request"

	// KindSession is a recorded SSH session.
	KindSession = "session"

	// KindSSHSession is an active SSH session.
	KindSSHSession = "ssh_session"

	// KindWebSession is a web session resource
	KindWebSession = "web_session"

	// KindAppSession represents an application specific web session.
	KindAppSession = "app_session"

	// KindEvent is structured audit logging event
	KindEvent = "event"

	// KindAuthServer is auth server resource
	KindAuthServer = "auth_server"

	// KindProxy is proxy resource
	KindProxy = "proxy"

	// KindNode is node resource
	KindNode = "node"

	// KindAppServer is an application server resource.
	KindAppServer = "app_server"

	// KindDatabaseServer is a database proxy server resource.
	KindDatabaseServer = "db_server"

	// KindToken is a provisioning token resource
	KindToken = "token"

	// KindCertAuthority is a certificate authority resource
	KindCertAuthority = "cert_authority"

	// KindReverseTunnel is a reverse tunnel connection
	KindReverseTunnel = "tunnel"

	// KindOIDCConnector is a OIDC connector resource
	KindOIDCConnector = "oidc"

	// KindSAMLConnector is a SAML connector resource
	KindSAMLConnector = "saml"

	// KindGithubConnector is Github OAuth2 connector resource
	KindGithubConnector = "github"

	// KindConnectors is a shortcut for all authentication connector
	KindConnectors = "connectors"

	// KindClusterAuthPreference is the type of authentication for this cluster.
	KindClusterAuthPreference = "cluster_auth_preference"

	// MetaNameClusterAuthPreference is the type of authentication for this cluster.
	MetaNameClusterAuthPreference = "cluster-auth-preference"

	// KindClusterConfig is the resource that holds cluster level configuration.
	KindClusterConfig = "cluster_config"

	// KindSemaphore is the resource that provides distributed semaphore functionality
	KindSemaphore = "semaphore"

	// MetaNameClusterConfig is the exact name of the cluster config singleton resource.
	MetaNameClusterConfig = "cluster-config"

	// KindClusterName is a type of configuration resource that contains the cluster name.
	KindClusterName = "cluster_name"

	// MetaNameClusterName is the name of a configuration resource for cluster name.
	MetaNameClusterName = "cluster-name"

	// KindStaticTokens is a type of configuration resource that contains static tokens.
	KindStaticTokens = "static_tokens"

	// MetaNameStaticTokens is the name of a configuration resource for static tokens.
	MetaNameStaticTokens = "static-tokens"

	// KindTrustedCluster is a resource that contains trusted cluster configuration.
	KindTrustedCluster = "trusted_cluster"

	// KindAuthConnector allows access to OIDC and SAML connectors.
	KindAuthConnector = "auth_connector"

	// KindTunnelConnection specifies connection of a reverse tunnel to proxy
	KindTunnelConnection = "tunnel_connection"

	// KindRemoteCluster represents remote cluster connected via reverse tunnel
	// to proxy
	KindRemoteCluster = "remote_cluster"

	// KindResetPasswordToken is a token used to change user passwords
	KindResetPasswordToken = "user_token"

	// KindResetPasswordTokenSecrets is reset password token secrets
	KindResetPasswordTokenSecrets = "reset_password_token_secrets"

	// KindIdentity is local on disk identity resource
	KindIdentity = "identity"

	// KindState is local on disk process state
	KindState = "state"

	// KindKubeService is a kubernetes service resource
	KindKubeService = "kube_service"

	// KindMFADevice is an MFA device for a user.
	KindMFADevice = "mfa_device"

	// V3 is the third version of resources.
	V3 = "v3"

	// V2 is the second version of resources.
	V2 = "v2"

	// V1 is the first version of resources. Note: The first version was
	// not explicitly versioned.
	V1 = "v1"
)

const (
	// VerbList is used to list all objects. Does not imply the ability to read a single object.
	VerbList = "list"

	// VerbCreate is used to create an object.
	VerbCreate = "create"

	// VerbRead is used to read a single object.
	VerbRead = "read"

	// VerbReadNoSecrets is used to read a single object without secrets.
	VerbReadNoSecrets = "readnosecrets"

	// VerbUpdate is used to update an object.
	VerbUpdate = "update"

	// VerbDelete is used to remove an object.
	VerbDelete = "delete"

	// VerbRotate is used to rotate certificate authorities
	// used only internally
	VerbRotate = "rotate"
)

const (
	// RecordAtNode is the default. Sessions are recorded at Teleport nodes.
	RecordAtNode = "node"

	// RecordAtProxy enables the recording proxy which intercepts and records
	// all sessions.
	RecordAtProxy = "proxy"

	// RecordOff is used to disable session recording completely.
	RecordOff = "off"

	// RecordAtNodeSync enables the nodes to stream sessions in sync mode
	// to the auth server
	RecordAtNodeSync = "node-sync"

	// RecordAtProxySync enables the recording proxy which intercepts and records
	// all sessions, streams the records synchronously
	RecordAtProxySync = "proxy-sync"
)

const (
	// HostKeyCheckYes is the default. The proxy will check the host key of the
	// target node it connects to.
	HostKeyCheckYes = "yes"

	// HostKeyCheckNo is used to disable host key checking. This is a insecure
	// settings which makes MITM possible with no indications, use with caution.
	HostKeyCheckNo = "no"
)

// TunnelType is the type of tunnel.
type TunnelType string

const (
	// NodeTunnel is a tunnel where the node connects to the proxy (dial back).
	NodeTunnel TunnelType = "node"

	// ProxyTunnel is a tunnel where a proxy connects to the proxy (trusted cluster).
	ProxyTunnel TunnelType = "proxy"

	// AppTunnel is a tunnel where the application proxy dials back to the proxy.
	AppTunnel TunnelType = "app"

	// KubeTunnel is a tunnel where the kubernetes service dials back to the proxy.
	KubeTunnel TunnelType = "kube"

	// DatabaseTunnel is a tunnel where a database proxy dials back to the proxy.
	DatabaseTunnel TunnelType = "db"
)
