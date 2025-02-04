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

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';
import { DiscoverEventResource } from 'teleport/services/userEvent';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { ResourceKind } from '../../Shared';
import { SelectResourceSpec } from './resources';

export const SAML_APPLICATIONS: SelectResourceSpec[] = [
  {
    id: DiscoverGuideId.ApplicationSamlGeneric,
    name: 'SAML Application (Generic)',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.Unspecified },
    keywords: ['saml', 'sso', 'application', 'idp'],
    icon: 'application',
    event: DiscoverEventResource.SamlApplication,
  },
  {
    id: DiscoverGuideId.ApplicationSamlGrafana,
    name: 'Grafana SAML',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.Grafana },
    keywords: ['saml', 'sso', 'application', 'idp', 'grafana'],
    icon: 'grafana',
    event: DiscoverEventResource.SamlApplication,
  },
  {
    id: DiscoverGuideId.ApplicationSamlWorkforceIdentityFederation,
    name: 'Workforce Identity Federation',
    kind: ResourceKind.SamlApplication,
    samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
    keywords: [
      'saml',
      'sso',
      'application',
      'idp',
      'gcp',
      'workforce',
      'federation',
    ],
    icon: 'googlecloud',
    event: DiscoverEventResource.SamlApplication,
  },
];
