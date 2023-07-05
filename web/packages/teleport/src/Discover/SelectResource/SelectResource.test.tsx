/**
 * Copyright 2023 Gravitational, Inc.
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

import { ResourceKind } from '../Shared';

import { sortResources } from './SelectResource';
import { ResourceSpec } from './types';

test('sortResources sorts resources alphabetically with guided resources first', () => {
  const sorted = sortResources(mockUnsorted);

  expect(sorted).toMatchObject(mockSorted);
});

const mockUnsorted: ResourceSpec[] = [
  {
    name: 'jenkins',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'grafana',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'linux',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'apple',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  // Guided
  {
    name: 'zapier',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
  {
    name: 'amazon',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
  {
    name: 'costco',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
];

const mockSorted: ResourceSpec[] = [
  {
    name: 'amazon',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
  {
    name: 'costco',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
  {
    name: 'zapier',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
  },
  {
    name: 'apple',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'grafana',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'jenkins',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
  {
    name: 'linux',
    kind: ResourceKind.Application,
    icon: 'Apple',
    event: null,
    keywords: 'test',
    hasAccess: true,
    unguidedLink: 'test.com',
  },
];
