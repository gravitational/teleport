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
  },
  {
    name: 'Debian 8+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'debian',
    Icon: icons.Linux,
  },
  {
    name: 'RHEL/CentOS 7+',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'rhel centos',
    Icon: icons.Linux,
  },
  {
    name: 'Amazon Linux 2',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'amazon linux',
    Icon: icons.Aws,
  },
  {
    name: 'macOS (Intel)',
    kind: ResourceKind.Server,
    keywords: baseServerKeywords + 'mac macos intel',
    Icon: icons.Apple,
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
  },
];

export const WINDOWS_DESKTOPS: ResourceSpec[] = [
  {
    name: 'Active Directory',
    kind: ResourceKind.Desktop,
    keywords: 'windows desktop active directory ad',
    Icon: icons.Windows,
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

export function getResourceTitles(r: ResourceSpec) {
  if (!r) {
    return {};
  }

  let pretitle;
  switch (r.kind) {
    case ResourceKind.Database:
      if (r.dbMeta) {
        switch (r.dbMeta.location) {
          case DatabaseLocation.AWS:
            pretitle = 'Amazon Web Services (AWS)';
            break;
          case DatabaseLocation.GCP:
            pretitle = 'Google Cloud Provider (GCP)';
            break;
          case DatabaseLocation.SelfHosted:
            pretitle = 'Self-Hosted';
            break;
          case DatabaseLocation.Azure:
            pretitle = 'Azure';
            break;
        }
      }
      break;
    case ResourceKind.Desktop:
      pretitle = 'Windows Desktop';
      break;
    case ResourceKind.Server:
      pretitle = 'Server';
      break;
  }

  return { pretitle, title: r.name };
}
