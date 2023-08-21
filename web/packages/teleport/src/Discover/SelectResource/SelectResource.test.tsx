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

import { Platform } from 'design/theme/utils';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import {
  ClusterResource,
  OnboardUserPreferences,
} from 'teleport/services/userPreferences/types';

import { ResourceKind } from '../Shared';

import { sortResources } from './SelectResource';
import { ResourceSpec } from './types';

const setUp = () => {
  jest.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue('Macintosh');
};

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
  setUp();
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
  beforeEach(() => {
    setUp();
  });

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

const osBasedList: ResourceSpec[] = [
  makeResourceSpec({ name: 'Aaaa' }),
  makeResourceSpec({ name: 'win', platform: Platform.PLATFORM_WINDOWS }),
  makeResourceSpec({ name: 'linux-2', platform: Platform.PLATFORM_LINUX }),
  makeResourceSpec({ name: 'mac', platform: Platform.PLATFORM_MACINTOSH }),
  makeResourceSpec({ name: 'linux-1', platform: Platform.PLATFORM_LINUX }),
];

describe('os sorted resources', () => {
  let OS;

  beforeEach(() => {
    OS = jest.spyOn(window.navigator, 'userAgent', 'get');
  });

  const testCases: {
    name: string;
    platform: Platform;
    expected: ResourceSpec[];
  }[] = [
    {
      name: 'running mac',
      platform: Platform.PLATFORM_MACINTOSH,
      expected: [
        // preferred first
        makeResourceSpec({
          name: 'mac',
          platform: Platform.PLATFORM_MACINTOSH,
        }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.PLATFORM_LINUX,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.PLATFORM_LINUX,
        }),
        makeResourceSpec({ name: 'win', platform: Platform.PLATFORM_WINDOWS }),
      ],
    },
    {
      name: 'running linux',
      platform: Platform.PLATFORM_LINUX,
      expected: [
        // preferred first
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.PLATFORM_LINUX,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.PLATFORM_LINUX,
        }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'mac',
          platform: Platform.PLATFORM_MACINTOSH,
        }),
        makeResourceSpec({ name: 'win', platform: Platform.PLATFORM_WINDOWS }),
      ],
    },
    {
      name: 'running windows',
      platform: Platform.PLATFORM_WINDOWS,
      expected: [
        // preferred first
        makeResourceSpec({ name: 'win', platform: Platform.PLATFORM_WINDOWS }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.PLATFORM_LINUX,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.PLATFORM_LINUX,
        }),
        makeResourceSpec({
          name: 'mac',
          platform: Platform.PLATFORM_MACINTOSH,
        }),
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    OS.mockReturnValue(testCase.platform);

    const actual = sortResources(osBasedList, makeDefaultUserPreferences());
    expect(actual).toMatchObject(testCase.expected);
  });

  test('does not prioritize os if the user does not have access', () => {
    const mockIn: ResourceSpec[] = [
      makeResourceSpec({
        name: 'macOs',
        platform: Platform.PLATFORM_MACINTOSH,
        hasAccess: false,
      }),
      makeResourceSpec({ name: 'Aaaa' }),
    ];
    OS.mockReturnValue(Platform.PLATFORM_MACINTOSH);

    const actual = sortResources(mockIn, makeDefaultUserPreferences());
    expect(actual).toMatchObject([
      makeResourceSpec({ name: 'Aaaa' }),
      makeResourceSpec({
        name: 'macOs',
        platform: Platform.PLATFORM_MACINTOSH,
        hasAccess: false,
      }),
    ]);
  });

  const oneOfEachList: ResourceSpec[] = [
    makeResourceSpec({
      name: 'no access but super matches',
      hasAccess: false,
      platform: Platform.PLATFORM_MACINTOSH,
      kind: ResourceKind.Server,
    }),
    makeResourceSpec({ name: 'guided' }),
    makeResourceSpec({ name: 'unguidedA', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'unguidedB', unguidedLink: 'test.com' }),
    makeResourceSpec({
      name: 'platform match',
      platform: Platform.PLATFORM_MACINTOSH,
    }),
    makeResourceSpec({ name: 'preferred', kind: ResourceKind.Server }),
  ];

  test('all logic together', () => {
    OS.mockReturnValue(Platform.PLATFORM_MACINTOSH);
    const preferences = makeDefaultUserPreferences();
    preferences.onboard = { preferredResources: [2] };

    const actual = sortResources(oneOfEachList, preferences);
    expect(actual).toMatchObject([
      // 1. OS
      makeResourceSpec({
        name: 'platform match',
        platform: Platform.PLATFORM_MACINTOSH,
      }),
      // 2. preferred
      makeResourceSpec({ name: 'preferred', kind: ResourceKind.Server }),
      // 3. guided
      makeResourceSpec({ name: 'guided' }),
      // 4. alpha
      makeResourceSpec({ name: 'unguidedA', unguidedLink: 'test.com' }),
      makeResourceSpec({ name: 'unguidedB', unguidedLink: 'test.com' }),
      // 5. no access is last
      makeResourceSpec({
        name: 'no access but super matches',
        hasAccess: false,
        platform: Platform.PLATFORM_MACINTOSH,
        kind: ResourceKind.Server,
      }),
    ]);
  });
});
