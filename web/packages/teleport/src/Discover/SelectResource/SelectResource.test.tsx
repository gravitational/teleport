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

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import {
  ClusterResource,
  OnboardUserPreferences,
} from 'teleport/services/userPreferences/types';

import { ResourceKind } from '../Shared';

import { sortResources } from './SelectResource';
import { ResourceSpec } from './types';

const makeResourceSpec = (
  overrides: Partial<ResourceSpec> = {}
): ResourceSpec => {
  return Object.assign(
    {
      name: '',
      kind: ResourceKind.Application,
      icon: '',
      event: null,
      keywords: '',
      hasAccess: true,
    },
    overrides
  );
};

test('sortResources without preferred resources, sorts resources alphabetically with guided resources first', () => {
  const mockIn: ResourceSpec[] = [
    // unguided
    makeResourceSpec({ name: 'jenkins', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'grafana', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'linux', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'apple', unguidedLink: 'test.com' }),
    // guided
    makeResourceSpec({ name: 'zapier' }),
    makeResourceSpec({ name: 'amazon' }),
    makeResourceSpec({ name: 'costco' }),
  ];

  const actual = sortResources(mockIn, makeDefaultUserPreferences());

  expect(actual).toMatchObject([
    // guided and alpha
    makeResourceSpec({ name: 'amazon' }),
    makeResourceSpec({ name: 'costco' }),
    makeResourceSpec({ name: 'zapier' }),
    // unguided and alpha
    makeResourceSpec({ name: 'apple', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'grafana', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'jenkins', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'linux', unguidedLink: 'test.com' }),
  ]);
});

const kindBasedList: ResourceSpec[] = [
  makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
  makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
  makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
  makeResourceSpec({
    name: 'echo',
    kind: ResourceKind.Kubernetes,
    unguidedLink: 'test.com',
  }),
  makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
  makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
  makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
  makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
  makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
  makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
  makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
  makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
];

describe('preferred resources', () => {
  const testCases: {
    name: string;
    preferred: OnboardUserPreferences;
    expected: ResourceSpec[];
  }[] = [
    {
      name: 'preferred server/ssh',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_SERVER_SSH],
      },
      expected: [
        // preferred first
        makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
        // alpha; guided before unguided
        makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
        makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
        makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
        makeResourceSpec({
          name: 'echo',
          kind: ResourceKind.Kubernetes,
          unguidedLink: 'test.com',
        }),
      ],
    },
    {
      name: 'preferred databases',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_DATABASES],
      },
      expected: [
        // preferred first
        makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
        // alpha; guided before unguided
        makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
        makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
        makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
        makeResourceSpec({
          name: 'echo',
          kind: ResourceKind.Kubernetes,
          unguidedLink: 'test.com',
        }),
      ],
    },
    {
      name: 'preferred windows',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_WINDOWS_DESKTOPS],
      },
      expected: [
        // preferred first
        makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
        makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
        // alpha; guided before unguided
        makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
        makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({
          name: 'echo',
          kind: ResourceKind.Kubernetes,
          unguidedLink: 'test.com',
        }),
      ],
    },
    {
      name: 'preferred applications',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_WEB_APPLICATIONS],
      },
      expected: [
        // preferred first
        makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
        // alpha; guided before unguided
        makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
        makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
        makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
        makeResourceSpec({
          name: 'echo',
          kind: ResourceKind.Kubernetes,
          unguidedLink: 'test.com',
        }),
      ],
    },
    {
      name: 'preferred kubernetes',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_KUBERNETES],
      },
      expected: [
        // preferred first; guided before unguided
        makeResourceSpec({ name: 'juliette', kind: ResourceKind.Kubernetes }),
        makeResourceSpec({
          name: 'echo',
          kind: ResourceKind.Kubernetes,
          unguidedLink: 'test.com',
        }),
        // alpha
        makeResourceSpec({ name: 'alpha', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'charlie', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'delta', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'foxtrot', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'golf', kind: ResourceKind.Application }),
        makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server }),
        makeResourceSpec({ name: 'india', kind: ResourceKind.Desktop }),
        makeResourceSpec({ name: 'kilo', kind: ResourceKind.Database }),
        makeResourceSpec({ name: 'lima', kind: ResourceKind.SamlApplication }),
        makeResourceSpec({ name: 'linux', kind: ResourceKind.Desktop }),
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const preferences = makeDefaultUserPreferences();
    preferences.onboard = testCase.preferred;
    const actual = sortResources(kindBasedList, preferences);

    expect(actual).toMatchObject(testCase.expected);
  });
});
