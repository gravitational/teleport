/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { DiscoverEventResource } from 'teleport/services/userEvent';

import { ResourceKind } from '../Shared';

import { ResourceSpec } from './types';

export const SAML_APPLICATIONS: ResourceSpec[] = [
  {
    name: 'SAML Application',
    kind: ResourceKind.SamlApplication,
    keywords: 'saml sso application idp',
    icon: 'Application',
    event: DiscoverEventResource.SamlApplication,
  },
  {
    name: 'SAML Application (Grafana)',
    kind: ResourceKind.SamlApplication,
    keywords: 'saml sso application idp grafana',
    icon: 'Grafana',
    event: DiscoverEventResource.SamlApplication,
  },
];
