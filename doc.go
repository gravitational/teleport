// Copyright 2021 Gravitational, Inc
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

/*
Gravitational Teleport is a modern SSH server for remotely accessing clusters
of Linux servers via SSH or HTTPS. It is intended to be used instead of sshd.

Teleport enables teams to easily adopt the best SSH practices like:

  - No need to distribute keys: Teleport uses certificate-based access with
    automatic expiration time.
  - Enforcement of 2nd factor authentication.
  - Cluster introspection: every Teleport node becomes a part of a cluster
    and is visible on the Web UI.
  - Record and replay SSH sessions for knowledge sharing and auditing purposes.
  - Collaboratively troubleshoot issues through session sharing.
  - Connect to clusters located behind firewalls without direct Internet
    access via SSH bastions.
  - Ability to integrate SSH credentials with your organization identities
    via OAuth (Google Apps, Github).
  - Keep the full audit log of all SSH sessions within a cluster.

Teleport web site:

	https://gravitational.com/teleport/

Teleport on Github:

	https://github.com/gravitational/teleport
*/
package teleport
