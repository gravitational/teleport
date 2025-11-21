/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Meta, StoryObj } from '@storybook/react-vite';

import { DemoTerminal as Component } from './DemoTerminal';

const text = `$ uname -a
Darwin TeleportMBP 24.3.0 Darwin Kernel Version 24.3.0: Thu Jan  2 20:24:16 PST 2025
$ tsh status
> Profile URL:        https://teleport-local.dev:3090
  Logged in as:       rav
  Cluster:            teleport-local
  Roles:              access, auto-users-access, connect-my-computer-rav, db, db-users-prod, db-users-staging, editor
  Logins:             some-nonexistent-user, root, boop, rav, custom-user, parallels, custom-user2
  Kubernetes:         enabled
  Kubernetes users:   minikube
  Kubernetes groups:  admins, viewers
  Valid until:        2025-03-21 04:39:52 +0100 CET [valid for 12h0m0s]
  Extensions:         login-ip, permit-agent-forwarding, permit-port-forwarding, permit-pty, private-key-policy

  Profile URL:        https://teleport-17-ent.asteroid.earth:443
  Logged in as:       ravicious
  Cluster:            teleport-17-ent.asteroid.earth
  Roles:              access, aws-console-access-iam, tf_vnet, windows_desktop_service
  Logins:             ravicious, ubuntu
  Kubernetes:         enabled
  Kubernetes groups:  system:masters
  GitHub username:    ravicious
  Valid until:        2025-03-21 02:39:42 +0100 CET [valid for 10h0m0s]
  Extensions:         login-ip, permit-agent-forwarding, permit-port-forwarding, permit-pty, private-key-policy
$ tsh proxy app api
Proxying connections to api on 127.0.0.1:65312
To avoid port randomization, you can choose the listening port using the --port flag.
⌷`;

const meta: Meta<typeof Component> = {
  title: 'Design/DemoTerminal',
  component: Component,
  args: {
    title: 'alice@TeleportMBP',
    text,
  },
};
export default meta;

export const DemoTerminal: StoryObj<typeof Component> = {};
