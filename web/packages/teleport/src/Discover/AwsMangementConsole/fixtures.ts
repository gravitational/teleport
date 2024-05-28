/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { App } from 'teleport/services/apps';

export const app: App = {
  kind: 'app',
  id: 'id',
  name: 'Jenkins',
  launchUrl: '',
  awsRoles: [],
  userGroups: [],
  samlApp: false,
  uri: 'https://jenkins.teleport-proxy.com',
  publicAddr: 'jenkins.teleport-proxy.com',
  description: 'This is a Jenkins app',
  awsConsole: true,
  labels: [
    { name: 'env', value: 'prod' },
    { name: 'cluster', value: 'one' },
  ],
  clusterId: 'one',
  fqdn: 'jenkins.one',
};
