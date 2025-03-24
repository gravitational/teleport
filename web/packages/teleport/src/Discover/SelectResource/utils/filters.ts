/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ResourceKind } from 'teleport/Discover/Shared';
import { AuthType } from 'teleport/services/user';

import { SelectResourceSpec } from '../resources';

export function filterBySupportedPlatformsAndAuthTypes(
  platform: Platform,
  authType: AuthType,
  resources: SelectResourceSpec[]
) {
  return resources.filter(resource => {
    const resourceSupportsPlatform =
      !resource.supportedPlatforms?.length ||
      resource.supportedPlatforms.includes(platform);

    const resourceSupportsAuthType =
      !resource.supportedAuthTypes?.length ||
      resource.supportedAuthTypes.includes(authType);

    return resourceSupportsPlatform && resourceSupportsAuthType;
  });
}

export const resourceTypeOptions = [
  { value: 'app', label: 'Applications' },
  { value: 'db', label: 'Database' },
  { value: 'desktops', label: 'Desktops' },
  { value: 'kube', label: 'Kubernetes' },
  { value: 'server', label: 'SSH' },
] as const satisfies { value: string; label: string }[];

type ResourceType = Extract<
  (typeof resourceTypeOptions)[number],
  { value: string }
>['value'];

export const hostingPlatformOptions = [
  { value: 'aws', label: 'Amazon Web Services (AWS)' },
  { value: 'azure', label: 'Microsoft Azure' },
  { value: 'gcp', label: 'Google Cloud Services (GCP)' },
  { value: 'self-hosted', label: 'Self-Hosted' },
] as const satisfies { value: string; label: string }[];

type HostingPlatform = Extract<
  (typeof hostingPlatformOptions)[number],
  { value: string }
>['value'];

export type Filters = {
  resourceTypes?: ResourceType[];
  hostingPlatforms?: HostingPlatform[];
};

export function filterResources(
  resources: SelectResourceSpec[],
  filters: Filters
) {
  if (
    !resources.length &&
    !filters.resourceTypes &&
    !filters.hostingPlatforms
  ) {
    return resources;
  }

  let filtered = [...resources];
  if (filters.resourceTypes.length) {
    const resourceTypes = filters.resourceTypes;
    filtered = filtered.filter(r => {
      if (
        resourceTypes.includes('app') &&
        (r.kind === ResourceKind.Application ||
          r.kind === ResourceKind.SamlApplication)
      ) {
        return true;
      }
      if (resourceTypes.includes('db') && r.kind === ResourceKind.Database) {
        return true;
      }
      if (
        resourceTypes.includes('desktops') &&
        r.kind === ResourceKind.Desktop
      ) {
        return true;
      }
      if (
        resourceTypes.includes('kube') &&
        r.kind === ResourceKind.Kubernetes
      ) {
        return true;
      }
      if (
        resourceTypes.includes('server') &&
        (r.kind === ResourceKind.Server ||
          r.kind === ResourceKind.ConnectMyComputer)
      ) {
        return true;
      }
    });
  }

  if (filters.hostingPlatforms.length) {
    const hostingPlatforms = filters.hostingPlatforms;
    filtered = filtered.filter(r => {
      if (
        hostingPlatforms.includes('aws') &&
        r.keywords.some(k => k.toLowerCase().includes('aws'))
      ) {
        return true;
      }
      if (
        hostingPlatforms.includes('azure') &&
        r.keywords.some(k => k.toLowerCase().includes('azure'))
      ) {
        return true;
      }
      if (
        hostingPlatforms.includes('gcp') &&
        r.keywords.some(k => k.toLowerCase().includes('gcp'))
      ) {
        return true;
      }
      if (
        hostingPlatforms.includes('self-hosted') &&
        r.keywords.some(k => k.toLowerCase().includes('self-hosted'))
      ) {
        return true;
      }
    });
  }

  return filtered;
}
