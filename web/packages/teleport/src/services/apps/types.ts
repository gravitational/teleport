/**
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

import { AwsRole } from 'shared/services/apps';

import { ResourceLabel } from 'teleport/services/agents';

export interface App {
  kind: 'app';
  id: string;
  name: string;
  description: string;
  uri: string;
  publicAddr: string;
  labels: ResourceLabel[];
  clusterId: string;
  launchUrl: string;
  fqdn: string;
  awsRoles: AwsRole[];
  awsConsole: boolean;
  isCloudOrTcpEndpoint?: boolean;
  // addrWithProtocol can either be a public address or
  // if public address wasn't defined, fallback to uri
  addrWithProtocol?: string;
  friendlyName?: string;
  userGroups: UserGroupAndDescription[];
  // samlApp is whether the application is a SAML Application (Service Provider).
  samlApp: boolean;
  // samlAppSsoUrl is the URL that triggers IdP-initiated SSO for SAML Application;
  samlAppSsoUrl?: string;
}

export type UserGroupAndDescription = {
  name: string;
  description: string;
};
