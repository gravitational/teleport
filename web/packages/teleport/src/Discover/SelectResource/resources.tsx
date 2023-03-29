/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { DiscoverEventResource } from 'teleport/services/userEvent';

import { ResourceKind } from '../Shared/ResourceKind';

import { DATABASES, DATABASES_UNGUIDED } from './databases';
import { ResourceSpec, DatabaseLocation } from './types';
import { icons } from './icons';

const baseServerKeywords = 'server node';
export const SERVERS: ResourceSpec[] = [
  {
    name: 'Ubuntu 14.04+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'ubuntu',
    Icon: icons.Linux,
    event: DiscoverEventResource.Server,
  },
  {
    name: 'Debian 8+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'debian',
    Icon: icons.Linux,
    event: DiscoverEventResource.Server,
  },
  {
    name: 'RHEL/CentOS 7+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'rhel centos',
    Icon: icons.Linux,
    event: DiscoverEventResource.Server,
  },
  {
    name: 'Amazon Linux 2',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'amazon linux',
    Icon: icons.Aws,
    event: DiscoverEventResource.Server,
  },
  {
    name: 'macOS (Intel)',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'mac macos intel',
    Icon: icons.Apple,
    event: DiscoverEventResource.Server,
  },
];

export const APPLICATIONS: ResourceSpec[] = [
  {
    name: 'Application',
    kind: ResourceKind.Application,
    keywords: 'application',
    Icon: icons.Application,
    unguidedLink:
      'https://goteleport.com/docs/application-access/getting-started/',
    event: DiscoverEventResource.ApplicationHttp,
  },
];

export const WINDOWS_DESKTOPS: ResourceSpec[] = [
  {
    name: 'Active Directory',
    kind: ResourceKind.Desktop,
    keywords: 'windows desktop active directory ad',
    Icon: icons.Windows,
    event: DiscoverEventResource.WindowsDesktop,
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
    Icon: icons.Kube,
    event: DiscoverEventResource.Kubernetes,
  },
];

export const RESOURCES: ResourceSpec[] = [
  ...APPLICATIONS,
  ...KUBERNETES,
  ...WINDOWS_DESKTOPS,
  ...SERVERS,
  ...DATABASES,
  ...DATABASES_UNGUIDED,
];

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
      }
      break;
    case ResourceKind.Desktop:
      return 'Windows Desktop';
    case ResourceKind.Server:
      return 'Server';
  }

  return '';
}
