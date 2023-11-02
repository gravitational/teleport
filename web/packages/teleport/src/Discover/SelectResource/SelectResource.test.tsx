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

const t_Application_NoAccess = makeResourceSpec({
  name: 'tango',
  kind: ResourceKind.Application,
  hasAccess: false,
});
const u_Database_NoAccess = makeResourceSpec({
  name: 'uniform',
  kind: ResourceKind.Database,
  hasAccess: false,
});
const v_Desktop_NoAccess = makeResourceSpec({
  name: 'victor',
  kind: ResourceKind.Desktop,
  hasAccess: false,
});
const w_Kubernetes_NoAccess = makeResourceSpec({
  name: 'whiskey',
  kind: ResourceKind.Kubernetes,
  hasAccess: false,
});
const x_Server_NoAccess = makeResourceSpec({
  name: 'xray',
  kind: ResourceKind.Server,
  hasAccess: false,
});
const y_Saml_NoAccess = makeResourceSpec({
  name: 'yankee',
  kind: ResourceKind.SamlApplication,
  hasAccess: false,
});
const z_Discovery_NoAccess = makeResourceSpec({
  name: 'zulu',
  kind: ResourceKind.Discovery,
  hasAccess: false,
});

const NoAccessList: ResourceSpec[] = [
  t_Application_NoAccess,
  u_Database_NoAccess,
  v_Desktop_NoAccess,
  w_Kubernetes_NoAccess,
  x_Server_NoAccess,
  y_Saml_NoAccess,
  z_Discovery_NoAccess,
];

const c_Application = makeResourceSpec({
  name: 'charlie',
  kind: ResourceKind.Application,
});
const a_Database = makeResourceSpec({
  name: 'alpha',
  kind: ResourceKind.Database,
});
const l_Desktop = makeResourceSpec({
  name: 'linux',
  kind: ResourceKind.Desktop,
});
const e_Kubernetes_unguided = makeResourceSpec({
  name: 'echo',
  kind: ResourceKind.Kubernetes,
  unguidedLink: 'test.com',
});
const f_Server = makeResourceSpec({
  name: 'foxtrot',
  kind: ResourceKind.Server,
});
const d_Saml = makeResourceSpec({
  name: 'delta',
  kind: ResourceKind.SamlApplication,
});
const g_Application = makeResourceSpec({
  name: 'golf',
  kind: ResourceKind.Application,
});
const k_Database = makeResourceSpec({
  name: 'kilo',
  kind: ResourceKind.Database,
});
const i_Desktop = makeResourceSpec({
  name: 'india',
  kind: ResourceKind.Desktop,
});
const j_Kubernetes = makeResourceSpec({
  name: 'juliette',
  kind: ResourceKind.Kubernetes,
});
const h_Server = makeResourceSpec({ name: 'hotel', kind: ResourceKind.Server });
const l_Saml = makeResourceSpec({
  name: 'lima',
  kind: ResourceKind.SamlApplication,
});

const kindBasedList: ResourceSpec[] = [
  c_Application,
  a_Database,
  t_Application_NoAccess,
  l_Desktop,
  e_Kubernetes_unguided,
  u_Database_NoAccess,
  f_Server,
  w_Kubernetes_NoAccess,
  d_Saml,
  v_Desktop_NoAccess,
  g_Application,
  x_Server_NoAccess,
  k_Database,
  i_Desktop,
  z_Discovery_NoAccess,
  j_Kubernetes,
  h_Server,
  y_Saml_NoAccess,
  l_Saml,
];

describe('preferred resources', () => {
  beforeEach(() => {
    setUp();
  });

  const testCases: {
    name: string;
    preferred: ClusterResource[];
    expected: ResourceSpec[];
  }[] = [
    {
      name: 'preferred server/ssh',
      preferred: [ClusterResource.RESOURCE_SERVER_SSH],
      expected: [
        // preferred first
        f_Server,
        h_Server,
        // alpha; guided before unguided
        a_Database,
        c_Application,
        d_Saml,
        g_Application,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred databases',
      preferred: [ClusterResource.RESOURCE_DATABASES],
      expected: [
        // preferred first
        a_Database,
        k_Database,
        // alpha; guided before unguided
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred windows',
      preferred: [ClusterResource.RESOURCE_WINDOWS_DESKTOPS],
      expected: [
        // preferred first
        i_Desktop,
        l_Desktop,
        // alpha; guided before unguided
        a_Database,
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        j_Kubernetes,
        k_Database,
        l_Saml,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred applications',
      preferred: [ClusterResource.RESOURCE_WEB_APPLICATIONS],
      expected: [
        // preferred first
        c_Application,
        g_Application,
        // alpha; guided before unguided
        a_Database,
        d_Saml,
        f_Server,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred kubernetes',
      preferred: [ClusterResource.RESOURCE_KUBERNETES],
      expected: [
        // preferred first; guided before unguided
        j_Kubernetes,
        e_Kubernetes_unguided,
        // alpha
        a_Database,
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_Desktop,
        // no access is last
        ...NoAccessList,
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const preferences = makeDefaultUserPreferences();
    preferences.onboard.preferredResources = testCase.preferred;
    const actual = sortResources(kindBasedList, preferences);

    expect(actual).toMatchObject(testCase.expected);
  });
});

describe('marketing params', () => {
  beforeEach(() => {
    setUp();
  });

  const testCases: {
    name: string;
    preferred: OnboardUserPreferences;
    expected: ResourceSpec[];
  }[] = [
    {
      name: 'marketing params instead of preferred resources',
      preferred: {
        preferredResources: [ClusterResource.RESOURCE_WEB_APPLICATIONS],
        marketingParams: {
          campaign: 'kubernetes',
          source: '',
          medium: '',
          intent: '',
        },
      },
      expected: [
        // marketing params first; no preferred priority, guided before unguided
        j_Kubernetes,
        e_Kubernetes_unguided,
        // alpha
        a_Database,
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_Desktop,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'param server/ssh',
      preferred: {
        preferredResources: [],
        marketingParams: {
          campaign: 'ssh',
          source: '',
          medium: '',
          intent: '',
        },
      },
      expected: [
        // preferred first
        f_Server,
        h_Server,
        // alpha; guided before unguided
        a_Database,
        c_Application,
        d_Saml,
        g_Application,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'param databases',
      preferred: {
        preferredResources: [],
        marketingParams: {
          campaign: '',
          source: 'database',
          medium: '',
          intent: '',
        },
      },
      expected: [
        // preferred first
        a_Database,
        k_Database,
        // alpha; guided before unguided
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'param windows',
      preferred: {
        preferredResources: [],
        marketingParams: {
          campaign: '',
          source: '',
          medium: 'windows',
          intent: '',
        },
      },
      expected: [
        // preferred first
        i_Desktop,
        l_Desktop,
        // alpha; guided before unguided
        a_Database,
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        j_Kubernetes,
        k_Database,
        l_Saml,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'param applications',
      preferred: {
        preferredResources: [],
        marketingParams: {
          campaign: '',
          source: '',
          medium: '',
          intent: 'application',
        },
      },
      expected: [
        // preferred first
        c_Application,
        g_Application,
        // alpha; guided before unguided
        a_Database,
        d_Saml,
        f_Server,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_Desktop,
        e_Kubernetes_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'param kubernetes',
      preferred: {
        preferredResources: [],
        marketingParams: {
          campaign: '',
          source: '',
          medium: 'k8s',
          intent: '',
        },
      },
      expected: [
        // preferred first; guided before unguided
        j_Kubernetes,
        e_Kubernetes_unguided,
        // alpha
        a_Database,
        c_Application,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_Desktop,
        // no access is last
        ...NoAccessList,
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
  makeResourceSpec({
    name: 'no-linux-1',
    platform: Platform.PLATFORM_LINUX,
    hasAccess: false,
  }),
  makeResourceSpec({ name: 'win', platform: Platform.PLATFORM_WINDOWS }),
  makeResourceSpec({ name: 'linux-2', platform: Platform.PLATFORM_LINUX }),
  makeResourceSpec({
    name: 'no-mac',
    platform: Platform.PLATFORM_MACINTOSH,
    hasAccess: false,
  }),
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
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.PLATFORM_LINUX,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.PLATFORM_MACINTOSH,
          hasAccess: false,
        }),
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
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.PLATFORM_LINUX,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.PLATFORM_MACINTOSH,
          hasAccess: false,
        }),
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
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.PLATFORM_LINUX,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.PLATFORM_MACINTOSH,
          hasAccess: false,
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
    preferences.onboard = {
      preferredResources: [2],
      marketingParams: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
    };

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
