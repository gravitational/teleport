/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
