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

import { DiscoverEventResource } from 'teleport/services/userEvent';

import { ResourceKind } from '../Shared';

import { ResourceSpec, SamlServiceProviderPreset } from './types';

export const SAML_APPLICATIONS: ResourceSpec[] = [
  {
    name: 'SAML Application',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.Unspecified },
    keywords: 'saml sso application idp',
    icon: 'Application',
    event: DiscoverEventResource.SamlApplication,
  },
  {
    name: 'Grafana',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.Grafana },
    keywords: 'saml sso application idp grafana',
    icon: 'Grafana',
    event: DiscoverEventResource.SamlApplication,
  },
  {
    name: 'Workforce Identity Federation',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
    keywords: 'saml sso application idp gcp workforce federation',
    icon: 'Gcp',
    event: DiscoverEventResource.SamlApplication,
  },
];
