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

import { Platform } from 'design/platform';

import { DiscoverEventResource } from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import { ResourceKind } from '../Shared/ResourceKind';

import {
  DATABASES,
  DATABASES_UNGUIDED,
  DATABASES_UNGUIDED_DOC,
} from './databases';
import {
  ResourceSpec,
  DatabaseLocation,
  DatabaseEngine,
  ServerLocation,
} from './types';
import { SAML_APPLICATIONS } from './resourcesE';

const baseServerKeywords = 'server node';
export const SERVERS: ResourceSpec[] = [
  {
    name: 'Ubuntu 14.04+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'ubuntu',
    icon: 'Linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'Debian 8+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'debian',
    icon: 'Linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'RHEL/CentOS 7+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'rhel centos',
    icon: 'Linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'Amazon Linux 2/2023',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'amazon linux',
    icon: 'Aws',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'macOS',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'mac macos intel silicone apple',
    icon: 'Apple',
    event: DiscoverEventResource.Server,
    platform: Platform.macOS,
  },
  {
    name: 'EC2 Instance',
    kind: ResourceKind.Server,
    keywords:
      baseServerKeywords + 'ec2 instance connect endpoint aws amazon eice',
    icon: 'Aws',
    event: DiscoverEventResource.Ec2Instance,
    nodeMeta: { location: ServerLocation.Aws },
  },
  {
    name: 'Connect My Computer',
    kind: ResourceKind.ConnectMyComputer,
    keywords: baseServerKeywords + 'connect my computer',
    icon: 'Laptop',
    event: DiscoverEventResource.Server,
    supportedPlatforms: [Platform.macOS, Platform.Linux],
    supportedAuthTypes: ['local', 'passwordless'],
  },
];

export const APPLICATIONS: ResourceSpec[] = [
  {
    name: 'Application',
    kind: ResourceKind.Application,
    keywords: 'application',
    icon: 'Application',
    isDialog: true,
    event: DiscoverEventResource.ApplicationHttp,
  },
];

export const WINDOWS_DESKTOPS: ResourceSpec[] = [
  {
    name: 'Active Directory',
    kind: ResourceKind.Desktop,
    keywords: 'windows desktop active directory ad',
    icon: 'Windows',
    event: DiscoverEventResource.WindowsDesktop,
    platform: Platform.Windows,
  },
  // {
  //   name: 'Non Active Directory',
  //   kind: ResourceKind.Desktop,
  //   keywords: 'windows desktop non-ad',
  //   Icon: iconLookup.Windows,
  //   comingSoon: true,
  // },
];

export const KUBERNETES: ResourceSpec[] = [
  {
    name: 'Kubernetes',
    kind: ResourceKind.Kubernetes,
    keywords: 'kubernetes cluster kubes',
    icon: 'Kube',
    event: DiscoverEventResource.Kubernetes,
  },
];

const BASE_RESOURCES: ResourceSpec[] = [
  ...APPLICATIONS,
  ...KUBERNETES,
  ...WINDOWS_DESKTOPS,
  ...SERVERS,
  ...DATABASES,
  ...DATABASES_UNGUIDED,
  ...DATABASES_UNGUIDED_DOC,
];

export const RESOURCES = !cfg.isEnterprise
  ? BASE_RESOURCES
  : [...BASE_RESOURCES, ...SAML_APPLICATIONS];

export function getResourcePretitle(r: ResourceSpec) {
  if (!r) {
    return {};
  }

  switch (r.kind) {
    case ResourceKind.Database:
      if (r.dbMeta) {
        switch (r.dbMeta.location) {
          case DatabaseLocation.Aws:
            return 'Amazon Web Services (AWS)';
          case DatabaseLocation.Gcp:
            return 'Google Cloud Provider (GCP)';
          case DatabaseLocation.SelfHosted:
            return 'Self-Hosted';
          case DatabaseLocation.Azure:
            return 'Azure';
          case DatabaseLocation.Microsoft:
            return 'Microsoft';
        }

        if (r.dbMeta.engine === DatabaseEngine.Doc) {
          return 'Database';
        }
      }
      break;
    case ResourceKind.Desktop:
      return 'Windows Desktop';
    case ResourceKind.Server:
      if (r.nodeMeta?.location === ServerLocation.Aws) {
        return 'Amazon Web Services (AWS)';
      }
      return 'Server';
  }

  return '';
}
