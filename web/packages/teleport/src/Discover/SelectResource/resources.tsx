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

import { assertUnreachable } from 'shared/utils/assertUnreachable';

import {
  DiscoverDiscoveryConfigMethod,
  DiscoverEventResource,
} from 'teleport/services/userEvent';

import { ResourceKind } from '../Shared/ResourceKind';

import {
  DATABASES,
  DATABASES_UNGUIDED,
  DATABASES_UNGUIDED_DOC,
} from './databases';
import {
  DatabaseEngine,
  DatabaseLocation,
  KubeLocation,
  ResourceSpec,
  ServerLocation,
} from './types';

const baseServerKeywords = ['server', 'node', 'ssh'];
const awsKeywords = ['aws', 'amazon', 'amazon web services'];
const kubeKeywords = ['kubernetes', 'k8s', 'kubes', 'cluster'];

export const SERVERS: ResourceSpec[] = [
  {
    name: 'Ubuntu 14.04+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'ubuntu', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'Debian 8+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'debian', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'RHEL/CentOS 7+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'rhel', 'redhat', 'centos', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'Amazon Linux 2/2023',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'amazon', 'linux'],
    icon: 'aws',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    name: 'macOS',
    kind: ResourceKind.Server,
    keywords: [
      ...baseServerKeywords,
      'mac',
      'macos',
      'osx',
      'intel',
      'silicon',
      'apple',
    ],
    icon: 'apple',
    event: DiscoverEventResource.Server,
    platform: Platform.macOS,
  },
  {
    name: 'EC2 Auto Enrollment',
    kind: ResourceKind.Server,
    keywords: [
      ...baseServerKeywords,
      ...awsKeywords,
      'ec2',
      'instance',
      'simple systems manager',
      'ssm',
      'auto enrollment',
    ],
    icon: 'aws',
    event: DiscoverEventResource.Ec2Instance,
    nodeMeta: {
      location: ServerLocation.Aws,
      discoveryConfigMethod: DiscoverDiscoveryConfigMethod.AwsEc2Ssm,
    },
  },
  {
    name: 'Connect My Computer',
    kind: ResourceKind.ConnectMyComputer,
    keywords: [
      ...baseServerKeywords,
      'connect my computer',
      'macos',
      'osx',
      'linux',
    ],
    icon: 'laptop',
    event: DiscoverEventResource.Server,
    supportedPlatforms: [Platform.macOS, Platform.Linux],
    supportedAuthTypes: ['local', 'passwordless'],
  },
];

export const APPLICATIONS: ResourceSpec[] = [
  {
    name: 'Application',
    kind: ResourceKind.Application,
    keywords: ['application'],
    icon: 'application',
    isDialog: true,
    event: DiscoverEventResource.ApplicationHttp,
  },
  {
    name: 'AWS CLI/Console Access',
    kind: ResourceKind.Application,
    keywords: [...awsKeywords, 'application', 'cli', 'console access'],
    icon: 'aws',
    event: DiscoverEventResource.ApplicationAwsConsole,
    appMeta: { awsConsole: true },
  },
];

export const WINDOWS_DESKTOPS: ResourceSpec[] = [
  {
    name: 'Active Directory Users',
    kind: ResourceKind.Desktop,
    keywords: ['windows', 'desktop', 'microsoft active directory', 'ad'],
    icon: 'windows',
    event: DiscoverEventResource.WindowsDesktop,
    unguidedLink:
      'https://goteleport.com/docs/desktop-access/active-directory/',
  },
  {
    name: 'Local Users',
    kind: ResourceKind.Desktop,
    keywords: ['windows', 'desktop', 'non-ad', 'local'],
    icon: 'windows',
    event: DiscoverEventResource.WindowsDesktopNonAD,
    unguidedLink: 'https://goteleport.com/docs/desktop-access/getting-started/',
  },
];

export const KUBERNETES: ResourceSpec[] = [
  {
    name: 'Kubernetes',
    kind: ResourceKind.Kubernetes,
    keywords: [...kubeKeywords],
    icon: 'kube',
    event: DiscoverEventResource.Kubernetes,
    kubeMeta: { location: KubeLocation.SelfHosted },
  },
  {
    name: 'EKS',
    kind: ResourceKind.Kubernetes,
    keywords: [...awsKeywords, ...kubeKeywords, 'eks', 'elastic', 'service'],
    icon: 'aws',
    event: DiscoverEventResource.KubernetesEks,
    kubeMeta: { location: KubeLocation.Aws },
  },
];

export const BASE_RESOURCES: ResourceSpec[] = [
  ...APPLICATIONS,
  ...KUBERNETES,
  ...WINDOWS_DESKTOPS,
  ...SERVERS,
  ...DATABASES,
  ...DATABASES_UNGUIDED,
  ...DATABASES_UNGUIDED_DOC,
];

export function getResourcePretitle(r: ResourceSpec) {
  if (!r) {
    return '';
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
    case ResourceKind.Kubernetes:
      if (r.kubeMeta) {
        switch (r.kubeMeta.location) {
          case KubeLocation.Aws:
            return 'Amazon Web Services (AWS)';
          case KubeLocation.SelfHosted:
            return 'Self-Hosted';
          default:
            assertUnreachable(r.kubeMeta.location);
        }
      }
      break;
    case ResourceKind.Server:
      if (r.nodeMeta?.location === ServerLocation.Aws) {
        return 'Amazon Web Services (AWS)';
      }
      return 'Server';
    case ResourceKind.SamlApplication:
      return 'SAML Application';
  }

  return '';
}
