/*
Copyright 2016 The Kubernetes Authors.

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

package proxy

import (
	"time"
)

const (
	// The SPDY subprotocol "v4.channel.k8s.io" is used for remote command
	// attachment/execution. It is the 4th version of the subprotocol and
	// adds support for exit codes.
	StreamProtocolV4Name = "v4.channel.k8s.io"

	// DefaultStreamCreationTimeout
	DefaultStreamCreationTimeout = 30 * time.Second
)

// These constants are for remote command execution and port forwarding and are
// used by both the client side and server side components.
//
// This is probably not the ideal place for them, but it didn't seem worth it
// to create pkg/exec and pkg/portforward just to contain a single file with
// constants in it.  Suggestions for more appropriate alternatives are
// definitely welcome!
const (
	// Enable stdin for remote command execution
	ExecStdinParam = "input"
	// Enable stdout for remote command execution
	ExecStdoutParam = "output"
	// Enable stderr for remote command execution
	ExecStderrParam = "error"
	// Enable TTY for remote command execution
	ExecTTYParam = "tty"
	// Command to run for remote command execution
	ExecCommandParam = "command"

	// Name of header that specifies stream type
	StreamType = "streamType"
	// Value for streamType header for stdin stream
	StreamTypeStdin = "stdin"
	// Value for streamType header for stdout stream
	StreamTypeStdout = "stdout"
	// Value for streamType header for stderr stream
	StreamTypeStderr = "stderr"
	// Value for streamType header for data stream
	StreamTypeData = "data"
	// Value for streamType header for error stream
	StreamTypeError = "error"
	// Value for streamType header for terminal resize stream
	StreamTypeResize = "resize"

	// Name of header that specifies the port being forwarded
	PortHeader = "port"
	// Name of header that specifies a request ID used to associate the error
	// and data streams for a single forwarded connection
	PortForwardRequestIDHeader = "requestID"

	// PortForwardProtocolV1Name is the subprotocol "portforward.k8s.io" is used for port forwarding
	PortForwardProtocolV1Name = "portforward.k8s.io"
)

const (
	// kubernetesResourcesKey is the key used to store the kubernetes resources
	// in the role.
	kubernetesResourcesKey = "kubernetes_resources"
)

const (
	// kubernete130BreakingChangeHint is a hint to help users fix the issue when
	// they are unable to exec into a pod due to RBAC rules not allowing the user
	// to access the pods/exec using  "get" verbs.
	kubernetes130BreakingChangeHint = "\n\n Kubernetes 1.30 switched to a new exec API that uses websockets.\n" +
		"To fix the issue, please ensure that your Kubernetes RBAC rules allow" +
		" the user to access the pods/exec using \"create\" and \"get\" verbs.\n" +
		"Please update your Kubernetes RBAC Roles and ClusterRoles with the following rule:\n" +
		`- apiGroups: [""]
resources: ["pods/exec"]
verbs: ["create", "get"]` + "\n"
)
