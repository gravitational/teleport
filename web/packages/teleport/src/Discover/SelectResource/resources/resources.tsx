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
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { ResourceKind } from '../../Shared/ResourceKind';
import {
  DatabaseEngine,
  DatabaseLocation,
  KubeLocation,
  ResourceSpec,
  ServerLocation,
} from '../types';
import {
  DATABASES,
  DATABASES_UNGUIDED,
  DATABASES_UNGUIDED_DOC,
} from './databases';
import {
  awsKeywords,
  baseServerKeywords,
  kubeKeywords,
  selfHostedKeywords,
} from './keywords';

export const SERVERS: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.ServerLinuxUbuntu,
    name: 'Ubuntu 18.04+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'ubuntu', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    id: DiscoverGuideId.ServerLinuxDebian,
    name: 'Debian 11+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'debian', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    id: DiscoverGuideId.ServerLinuxRhelCentos,
    name: 'RHEL 8+/CentOS Stream 9+',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'rhel', 'redhat', 'centos', 'linux'],
    icon: 'linux',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    id: DiscoverGuideId.ServerLinuxAmazon,
    name: 'Amazon Linux 2/2023',
    kind: ResourceKind.Server,
    keywords: [...baseServerKeywords, 'amazon', 'linux'],
    icon: 'aws',
    event: DiscoverEventResource.Server,
    platform: Platform.Linux,
  },
  {
    id: DiscoverGuideId.ServerMac,
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
    id: DiscoverGuideId.ServerAwsEc2Ssm,
    name: 'EC2 Auto Enrollment via SSM',
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
    id: DiscoverGuideId.ConnectMyComputer,
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

export const APPLICATIONS: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.ApplicationWebHttpProxy,
    name: 'Web Application',
    kind: ResourceKind.Application,
    keywords: ['application'],
    icon: 'application',
    isDialog: true,
    event: DiscoverEventResource.ApplicationHttp,
  },
  {
    id: DiscoverGuideId.ApplicationAwsCliConsole,
    name: 'AWS CLI/Console Access',
    kind: ResourceKind.Application,
    keywords: [...awsKeywords, 'application', 'cli', 'console access'],
    icon: 'aws',
    event: DiscoverEventResource.ApplicationAwsConsole,
    appMeta: { awsConsole: true },
  },
];

export const WINDOWS_DESKTOPS: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.WindowsDesktopsActiveDirectory,
    name: 'Active Directory Users',
    kind: ResourceKind.Desktop,
    keywords: ['windows', 'desktop', 'microsoft active directory', 'ad'],
    icon: 'windows',
    event: DiscoverEventResource.WindowsDesktop,
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/desktop-access/active-directory/',
  },
  {
    id: DiscoverGuideId.WindowsDesktopsLocal,
    name: 'Local Users',
    kind: ResourceKind.Desktop,
    keywords: ['windows', 'desktop', 'non-ad', 'local'],
    icon: 'windows',
    event: DiscoverEventResource.WindowsDesktopNonAD,
    unguidedLink:
      'https://goteleport.com/docs/enroll-resources/desktop-access/getting-started/',
  },
];

export const KUBERNETES: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.Kubernetes,
    name: 'Kubernetes',
    kind: ResourceKind.Kubernetes,
    keywords: [...kubeKeywords, ...selfHostedKeywords],
    icon: 'kube',
    event: DiscoverEventResource.Kubernetes,
    kubeMeta: { location: KubeLocation.SelfHosted },
  },
  {
    id: DiscoverGuideId.KubernetesAwsEks,
    name: 'EKS',
    kind: ResourceKind.Kubernetes,
    keywords: [...awsKeywords, ...kubeKeywords, 'eks', 'elastic', 'service'],
    icon: 'aws',
    event: DiscoverEventResource.KubernetesEks,
    kubeMeta: { location: KubeLocation.Aws },
  },
];

export const BASE_RESOURCES: SelectResourceSpec[] = [
  ...APPLICATIONS,
  ...KUBERNETES,
  ...WINDOWS_DESKTOPS,
  ...SERVERS,
  ...DATABASES,
  ...DATABASES_UNGUIDED,
  ...DATABASES_UNGUIDED_DOC,
];

export function getResourcePretitle(r: SelectResourceSpec) {
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
        if (
          r.id === DiscoverGuideId.DatabaseSnowflake ||
          r.id === DiscoverGuideId.DatabaseMongoAtlas
        ) {
          return 'Database as a Service';
        }
        if (r.id === DiscoverGuideId.DatabaseDynamicRegistration) {
          return 'Self-Hosted';
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
      return 'SSH';
    case ResourceKind.SamlApplication:
      if (
        r.id === DiscoverGuideId.ApplicationSamlGeneric ||
        r.id === DiscoverGuideId.ApplicationSamlGrafana
      ) {
        return 'Teleport as IDP';
      }
      return 'SAML Application';
    case ResourceKind.ConnectMyComputer:
      return 'SSH';
    case ResourceKind.Application:
      if (r.id === DiscoverGuideId.ApplicationAwsCliConsole) {
        return 'Amazon Web Services (AWS)';
      }
      if (r.id === DiscoverGuideId.ApplicationWebHttpProxy) {
        return 'HTTP Proxy';
      }
  }

  return '';
}

export type SelectResourceSpec = ResourceSpec & {
  id: DiscoverGuideId;
};
