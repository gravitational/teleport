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

import { within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { Platform, UserAgent } from 'design/platform';
import { render, screen, userEvent, waitFor } from 'design/utils/testing';
import {
  OnboardUserPreferences,
  Resource,
} from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { ContextProvider } from 'teleport/index';
import {
  allAccessAcl,
  createTeleportContext,
  noAccess,
} from 'teleport/mocks/contexts';
import { OnboardDiscover } from 'teleport/services/user';
import * as service from 'teleport/services/userPreferences/userPreferences';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import * as userUserContext from 'teleport/User/UserContext';
import { UserContextProvider } from 'teleport/User/UserContext';

import { ResourceKind } from '../Shared';
import { resourceKindToPreferredResource } from '../Shared/ResourceKind';
import { getGuideTileId } from '../testUtils';
import { SelectResourceSpec } from './resources';
import { SelectResource } from './SelectResource';
import {
  a_DatabaseAws,
  c_ApplicationGcp,
  d_Saml,
  e_KubernetesSelfHosted_unguided,
  f_Server,
  g_Application,
  h_Server,
  i_Desktop,
  j_Kubernetes,
  k_Database,
  kindBasedList,
  l_DesktopAzure,
  l_Saml,
  makeResourceSpec,
  NoAccessList,
} from './testUtils';
import { filterBySupportedPlatformsAndAuthTypes } from './utils/filters';
import { defaultPins } from './utils/pins';
import { sortResourcesByPreferences } from './utils/sort';

const setUp = () => {
  jest
    .spyOn(window.navigator, 'userAgent', 'get')
    .mockReturnValue(UserAgent.macOS);
};

/**
 * If the user has resources, Connect My Computer is not prioritized when sorting resources.
 */
const onboardDiscoverWithResources: OnboardDiscover = {
  hasResource: true,
  notified: true,
  hasVisited: true,
};
/**
 * If the user does not have resources, Connect My Computer is prioritized as long as it was not
 * filtered out based on supported platforms and auth types and the user either has no preferences
 * or prefers servers.
 */
const onboardDiscoverNoResources: OnboardDiscover = {
  hasResource: false,
  notified: true,
  hasVisited: false,
};

beforeEach(() => {
  jest.restoreAllMocks();
});

test('sortResourcesByPreferences without preferred resources, sorts resources alphabetically with guided resources first', () => {
  setUp();
  const mockIn: SelectResourceSpec[] = [
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

  const actual = sortResourcesByPreferences(
    mockIn,
    makeDefaultUserPreferences(),
    onboardDiscoverWithResources
  );

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

describe('preferred resources', () => {
  beforeEach(() => {
    setUp();
  });

  const testCases: {
    name: string;
    preferred: Resource[];
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'preferred server/ssh',
      preferred: [Resource.SERVER_SSH],
      expected: [
        // preferred first
        f_Server,
        h_Server,
        // alpha; guided before unguided
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        g_Application,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred databases',
      preferred: [Resource.DATABASES],
      expected: [
        // preferred first
        a_DatabaseAws,
        k_Database,
        // alpha; guided before unguided
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred windows',
      preferred: [Resource.WINDOWS_DESKTOPS],
      expected: [
        // preferred first
        i_Desktop,
        l_DesktopAzure,
        // alpha; guided before unguided
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        j_Kubernetes,
        k_Database,
        l_Saml,
        e_KubernetesSelfHosted_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred applications',
      preferred: [Resource.WEB_APPLICATIONS],
      expected: [
        // preferred first
        c_ApplicationGcp,
        g_Application,
        // alpha; guided before unguided
        a_DatabaseAws,
        d_Saml,
        f_Server,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
        // no access is last
        ...NoAccessList,
      ],
    },
    {
      name: 'preferred kubernetes',
      preferred: [Resource.KUBERNETES],
      expected: [
        // preferred first; guided before unguided
        j_Kubernetes,
        e_KubernetesSelfHosted_unguided,
        // alpha
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        // no access is last
        ...NoAccessList,
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const preferences = makeDefaultUserPreferences();
    preferences.onboard.preferredResources = testCase.preferred;
    const actual = sortResourcesByPreferences(
      kindBasedList,
      preferences,
      onboardDiscoverWithResources
    );

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
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'marketing params instead of preferred resources',
      preferred: {
        preferredResources: [Resource.WEB_APPLICATIONS],
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
        e_KubernetesSelfHosted_unguided,
        // alpha
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_DesktopAzure,
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
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        g_Application,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
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
        a_DatabaseAws,
        k_Database,
        // alpha; guided before unguided
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
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
        l_DesktopAzure,
        // alpha; guided before unguided
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        j_Kubernetes,
        k_Database,
        l_Saml,
        e_KubernetesSelfHosted_unguided,
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
        c_ApplicationGcp,
        g_Application,
        // alpha; guided before unguided
        a_DatabaseAws,
        d_Saml,
        f_Server,
        h_Server,
        i_Desktop,
        j_Kubernetes,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        e_KubernetesSelfHosted_unguided,
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
        e_KubernetesSelfHosted_unguided,
        // alpha
        a_DatabaseAws,
        c_ApplicationGcp,
        d_Saml,
        f_Server,
        g_Application,
        h_Server,
        i_Desktop,
        k_Database,
        l_Saml,
        l_DesktopAzure,
        // no access is last
        ...NoAccessList,
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const preferences = makeDefaultUserPreferences();
    preferences.onboard = testCase.preferred;
    const actual = sortResourcesByPreferences(
      kindBasedList,
      preferences,
      onboardDiscoverWithResources
    );

    expect(actual).toMatchObject(testCase.expected);
  });
});

const osBasedList: SelectResourceSpec[] = [
  makeResourceSpec({ name: 'Aaaa' }),
  makeResourceSpec({
    name: 'no-linux-1',
    platform: Platform.Linux,
    hasAccess: false,
  }),
  makeResourceSpec({ name: 'win', platform: Platform.Windows }),
  makeResourceSpec({ name: 'linux-2', platform: Platform.Linux }),
  makeResourceSpec({
    name: 'no-mac',
    platform: Platform.macOS,
    hasAccess: false,
  }),
  makeResourceSpec({ name: 'mac', platform: Platform.macOS }),
  makeResourceSpec({ name: 'linux-1', platform: Platform.Linux }),
];

describe('os sorted resources', () => {
  let OS;

  beforeEach(() => {
    OS = jest.spyOn(window.navigator, 'userAgent', 'get');
  });

  const testCases: {
    name: string;
    userAgent: UserAgent;
    expected: SelectResourceSpec[];
  }[] = [
    {
      name: 'running mac',
      userAgent: UserAgent.macOS,
      expected: [
        // preferred first
        makeResourceSpec({
          name: 'mac',
          platform: Platform.macOS,
        }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.Linux,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.Linux,
        }),
        makeResourceSpec({ name: 'win', platform: Platform.Windows }),
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.Linux,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.macOS,
          hasAccess: false,
        }),
      ],
    },
    {
      name: 'running linux',
      userAgent: UserAgent.Linux,
      expected: [
        // preferred first
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.Linux,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.Linux,
        }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'mac',
          platform: Platform.macOS,
        }),
        makeResourceSpec({ name: 'win', platform: Platform.Windows }),
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.Linux,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.macOS,
          hasAccess: false,
        }),
      ],
    },
    {
      name: 'running windows',
      userAgent: UserAgent.Windows,
      expected: [
        // preferred first
        makeResourceSpec({ name: 'win', platform: Platform.Windows }),
        // alpha
        makeResourceSpec({ name: 'Aaaa' }),
        makeResourceSpec({
          name: 'linux-1',
          platform: Platform.Linux,
        }),
        makeResourceSpec({
          name: 'linux-2',
          platform: Platform.Linux,
        }),
        makeResourceSpec({
          name: 'mac',
          platform: Platform.macOS,
        }),
        // no access, alpha
        makeResourceSpec({
          name: 'no-linux-1',
          platform: Platform.Linux,
          hasAccess: false,
        }),
        makeResourceSpec({
          name: 'no-mac',
          platform: Platform.macOS,
          hasAccess: false,
        }),
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    OS.mockReturnValue(testCase.userAgent);

    const actual = sortResourcesByPreferences(
      osBasedList,
      makeDefaultUserPreferences(),
      onboardDiscoverWithResources
    );
    expect(actual).toMatchObject(testCase.expected);
  });

  test('does not prioritize os if the user does not have access', () => {
    const mockIn: SelectResourceSpec[] = [
      makeResourceSpec({
        name: 'macOs',
        platform: Platform.macOS,
        hasAccess: false,
      }),
      makeResourceSpec({ name: 'Aaaa' }),
    ];
    OS.mockReturnValue(UserAgent.macOS);

    const actual = sortResourcesByPreferences(
      mockIn,
      makeDefaultUserPreferences(),
      onboardDiscoverWithResources
    );
    expect(actual).toMatchObject([
      makeResourceSpec({ name: 'Aaaa' }),
      makeResourceSpec({
        name: 'macOs',
        platform: Platform.macOS,
        hasAccess: false,
      }),
    ]);
  });

  const oneOfEachList: SelectResourceSpec[] = [
    makeResourceSpec({
      name: 'no access but super matches',
      hasAccess: false,
      platform: Platform.macOS,
      kind: ResourceKind.Server,
    }),
    makeResourceSpec({ name: 'guided' }),
    makeResourceSpec({ name: 'unguidedA', unguidedLink: 'test.com' }),
    makeResourceSpec({ name: 'unguidedB', unguidedLink: 'test.com' }),
    makeResourceSpec({
      name: 'platform match',
      platform: Platform.macOS,
    }),
    makeResourceSpec({ name: 'preferred', kind: ResourceKind.Server }),
  ];

  test('all logic together', () => {
    OS.mockReturnValue(UserAgent.macOS);
    const preferences = makeDefaultUserPreferences();
    preferences.onboard = {
      preferredResources: [
        resourceKindToPreferredResource(ResourceKind.Server),
      ],
      marketingParams: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
    };

    const actual = sortResourcesByPreferences(
      oneOfEachList,
      preferences,
      onboardDiscoverWithResources
    );
    expect(actual).toMatchObject([
      // 1. OS
      makeResourceSpec({
        name: 'platform match',
        platform: Platform.macOS,
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
        platform: Platform.macOS,
        kind: ResourceKind.Server,
      }),
    ]);
  });
});

describe('sorting Connect My Computer', () => {
  let OS: jest.SpyInstance;

  beforeEach(() => {
    OS = jest.spyOn(window.navigator, 'userAgent', 'get');
  });

  const connectMyComputer = makeResourceSpec({
    kind: ResourceKind.ConnectMyComputer,
    name: 'Connect My Computer',
  });
  const noAccessServerForMatchingPlatform = makeResourceSpec({
    name: 'no access but platform matches',
    hasAccess: false,
    platform: Platform.macOS,
    kind: ResourceKind.Server,
  });
  const guidedA = makeResourceSpec({ name: 'guided' });
  const guidedB = makeResourceSpec({ name: 'guidedB' });
  const unguidedA = makeResourceSpec({
    name: 'unguidedA',
    unguidedLink: 'test.com',
  });
  const unguidedB = makeResourceSpec({
    name: 'unguidedB',
    unguidedLink: 'test.com',
  });
  const platformMatch = makeResourceSpec({
    name: 'platform match',
    platform: Platform.macOS,
  });
  const server = makeResourceSpec({
    name: 'server',
    kind: ResourceKind.Server,
  });

  const oneOfEachList = [
    noAccessServerForMatchingPlatform,
    guidedB,
    unguidedB,
    guidedA,
    unguidedA,
    platformMatch,
    server,
    connectMyComputer,
  ];

  describe('prioritizing Connect My Computer', () => {
    it('puts the Connect My Computer resource as the first resource if the user has no preferences', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const actual = sortResourcesByPreferences(
        oneOfEachList,
        makeDefaultUserPreferences(),
        onboardDiscoverNoResources
      );

      expect(actual).toMatchObject([
        // 1. Connect My Computer
        connectMyComputer,
        // 2. OS
        platformMatch,
        // 3. guided
        guidedA,
        guidedB,
        server,
        // 4. alpha
        unguidedA,
        unguidedB,
        // 5. no access
        noAccessServerForMatchingPlatform,
      ]);
    });

    it('puts the Connect My Computer resource as the first resource if the user prefers servers', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const preferences = makeDefaultUserPreferences();
      preferences.onboard = {
        preferredResources: [
          resourceKindToPreferredResource(ResourceKind.Server),
        ],
        marketingParams: {
          campaign: '',
          source: '',
          medium: '',
          intent: '',
        },
      };

      const actual = sortResourcesByPreferences(
        oneOfEachList,
        preferences,
        onboardDiscoverNoResources
      );

      expect(actual).toMatchObject([
        // 1. Connect My Computer
        connectMyComputer,
        // 2. OS
        platformMatch,
        // 3. preferred
        server,
        // 4. guided
        guidedA,
        guidedB,
        // 5. alpha
        unguidedA,
        unguidedB,
        // 6. no access is last
        noAccessServerForMatchingPlatform,
      ]);
    });

    it('deprioritizes other server tiles of the matching platform within the guided resources if the user does not prefer servers', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const guidedServerForMatchingPlatformA = makeResourceSpec({
        name: 'guided server for matching platform A',
        kind: ResourceKind.Server,
        platform: Platform.macOS,
      });
      const guidedServerForMatchingPlatformB = makeResourceSpec({
        name: 'guided server for matching platform B',
        kind: ResourceKind.Server,
        platform: Platform.macOS,
      });
      const guidedServerForAnotherPlatform = makeResourceSpec({
        name: 'guided server for another platform',
        kind: ResourceKind.Server,
        platform: Platform.Linux,
      });

      const actual = sortResourcesByPreferences(
        [
          unguidedA,
          guidedServerForMatchingPlatformB,
          guidedServerForMatchingPlatformA,
          guidedServerForAnotherPlatform,
          connectMyComputer,
        ],
        makeDefaultUserPreferences(),
        onboardDiscoverNoResources
      );

      expect(actual).toMatchObject([
        connectMyComputer,
        guidedServerForAnotherPlatform,
        guidedServerForMatchingPlatformA,
        guidedServerForMatchingPlatformB,
        unguidedA,
      ]);
    });

    it('does not deprioritize server tiles of the matching platform if the user prefers servers,', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const guidedServerForMatchingPlatformA = makeResourceSpec({
        name: 'guided server for matching platform A',
        kind: ResourceKind.Server,
        platform: Platform.macOS,
      });
      const guidedServerForMatchingPlatformB = makeResourceSpec({
        name: 'guided server for matching platform B',
        kind: ResourceKind.Server,
        platform: Platform.macOS,
      });
      const guidedServerForAnotherPlatform = makeResourceSpec({
        name: 'guided server for another platform',
        kind: ResourceKind.Server,
        platform: Platform.Linux,
      });

      const preferences = makeDefaultUserPreferences();
      preferences.onboard = {
        preferredResources: [
          resourceKindToPreferredResource(ResourceKind.Server),
        ],
        marketingParams: {
          campaign: '',
          source: '',
          medium: '',
          intent: '',
        },
      };

      const actual = sortResourcesByPreferences(
        [
          unguidedA,
          guidedServerForMatchingPlatformB,
          guidedServerForMatchingPlatformA,
          guidedServerForAnotherPlatform,
          connectMyComputer,
        ],
        preferences,
        onboardDiscoverNoResources
      );

      expect(actual).toMatchObject([
        connectMyComputer,
        guidedServerForMatchingPlatformA,
        guidedServerForMatchingPlatformB,
        guidedServerForAnotherPlatform,
        unguidedA,
      ]);
    });
  });

  describe('deprioritizing Connect My Computer', () => {
    it('puts the Connect My Computer resource as the last guided resource if the user has resources', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const actual = sortResourcesByPreferences(
        oneOfEachList,
        makeDefaultUserPreferences(),
        onboardDiscoverWithResources
      );

      expect(actual).toMatchObject([
        // 1. OS
        platformMatch,
        // 2. guided
        guidedA,
        guidedB,
        server,
        // 3. Connect My Computer
        connectMyComputer,
        // 4. alpha
        unguidedA,
        unguidedB,
        // 5. no access
        noAccessServerForMatchingPlatform,
      ]);
    });

    it('puts the Connect My Computer resource as the last guided resource if the user has resources, even if the user prefers servers', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const preferences = makeDefaultUserPreferences();
      preferences.onboard = {
        preferredResources: [
          resourceKindToPreferredResource(ResourceKind.Server),
        ],
        marketingParams: {
          campaign: '',
          source: '',
          medium: '',
          intent: '',
        },
      };

      const actual = sortResourcesByPreferences(
        oneOfEachList,
        preferences,
        onboardDiscoverWithResources
      );

      expect(actual).toMatchObject([
        // 1. OS
        platformMatch,
        // 2. preferred
        server,
        // 2. guided
        guidedA,
        guidedB,
        // 3. Connect My Computer,
        connectMyComputer,
        // 4. alpha
        unguidedA,
        unguidedB,
        // 6. no access is last
        noAccessServerForMatchingPlatform,
      ]);
    });

    it('puts the Connect My Computer resource as the last guided resource if the user has no resources but they prefer other resources than servers', () => {
      OS.mockReturnValue(UserAgent.macOS);

      const databaseForAnotherPlatform = makeResourceSpec({
        name: 'database for another platform',
        kind: ResourceKind.Database,
        platform: Platform.Linux,
      });

      const preferences = makeDefaultUserPreferences();
      preferences.onboard = {
        preferredResources: [
          resourceKindToPreferredResource(ResourceKind.Database),
        ],
        marketingParams: {
          campaign: '',
          source: '',
          medium: '',
          intent: '',
        },
      };

      const actual = sortResourcesByPreferences(
        [...oneOfEachList, databaseForAnotherPlatform],
        preferences,
        onboardDiscoverNoResources
      );

      expect(actual).toMatchObject([
        // 1. OS
        platformMatch,
        // 2. preferred
        databaseForAnotherPlatform,
        // 2. guided
        guidedA,
        guidedB,
        server,
        // 3. Connect My Computer,
        connectMyComputer,
        // 4. alpha
        unguidedA,
        unguidedB,
        // 6. no access is last
        noAccessServerForMatchingPlatform,
      ]);
    });
  });
});

test('displays an info banner if lacking "all" permissions to add resources', async () => {
  jest.spyOn(userUserContext, 'useUser').mockReturnValue({
    preferences: makeDefaultUserPreferences(),
    updatePreferences: () => null,
    updateClusterPinnedResources: () => null,
    getClusterPinnedResources: () => null,
  });

  const ctx = createTeleportContext();
  ctx.storeUser.setState({ acl: { ...allAccessAcl, tokens: noAccess } });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SelectResource onSelect={() => {}} />
      </ContextProvider>
    </MemoryRouter>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/You cannot add new resources./i)
    ).toBeInTheDocument();
  });
});

test('add and remove pin, and rendering of default pins', async () => {
  jest
    .spyOn(window.navigator, 'userAgent', 'get')
    .mockReturnValue(UserAgent.macOS);

  const prefs = makeDefaultUserPreferences();
  jest.spyOn(service, 'getUserPreferences').mockResolvedValue(prefs);
  jest.spyOn(service, 'updateUserPreferences').mockResolvedValue(prefs);

  render(
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContext()}>
        <UserContextProvider>
          <SelectResource onSelect={() => {}} />
        </UserContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );

  await screen.findAllByTestId(/large-tile-/);

  // Default pins on initial render with no preferences set.
  let pinnedGuides = screen.queryAllByTestId(/large-tile-/);
  expect(pinnedGuides).toHaveLength(defaultPins.length);

  // Add pin.
  let snowflakeGuide = screen.getByTestId(
    getGuideTileId({ kind: ResourceKind.Database, title: 'snowflake' })
  );
  await userEvent.click(within(snowflakeGuide).getByTestId(/pin-button/i));
  pinnedGuides = screen.queryAllByTestId(/large-tile-/);
  expect(pinnedGuides).toHaveLength(defaultPins.length + 1);

  // Remove pin.
  snowflakeGuide = screen.getByTestId(
    getGuideTileId({
      kind: ResourceKind.Database,
      title: 'snowflake',
      size: 'large',
    })
  );
  await userEvent.click(within(snowflakeGuide).getByTestId(/pin-button/i));
  pinnedGuides = screen.queryAllByTestId(/large-tile-/);
  expect(pinnedGuides).toHaveLength(defaultPins.length);
});

test('does not display erorr banner if user has "some" permissions to add', async () => {
  jest.spyOn(userUserContext, 'useUser').mockReturnValue({
    preferences: makeDefaultUserPreferences(),
    updatePreferences: () => null,
    updateClusterPinnedResources: () => null,
    getClusterPinnedResources: () => null,
  });

  const ctx = createTeleportContext();
  ctx.storeUser.setState({ acl: { ...allAccessAcl } });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SelectResource onSelect={() => {}} />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(
    screen.queryByText(/You cannot add new resources./i)
  ).not.toBeInTheDocument();
});

describe('filterBySupportedPlatformsAndAuthTypes', () => {
  it('filters out resources based on supportedPlatforms', () => {
    const winAndLinux = makeResourceSpec({
      name: 'Filtered out with many supported platforms',
      supportedPlatforms: [Platform.Windows, Platform.Linux],
    });
    const win = makeResourceSpec({
      name: 'Filtered out with one supported platform',
      supportedPlatforms: [Platform.Windows],
    });
    const macosAndLinux = makeResourceSpec({
      name: 'Kept with many supported platforms',
      supportedPlatforms: [Platform.macOS, Platform.Linux],
    });
    const macos = makeResourceSpec({
      name: 'Kept with one supported platform',
      supportedPlatforms: [Platform.macOS],
    });

    const result = filterBySupportedPlatformsAndAuthTypes(
      Platform.macOS,
      'local',
      [winAndLinux, win, macosAndLinux, macos]
    );

    expect(result).toContain(macosAndLinux);
    expect(result).toContain(macos);
    expect(result).not.toContain(winAndLinux);
    expect(result).not.toContain(win);
  });

  it('does not filter out resources with supportedPlatforms and supportedAuthTypes that are missing or empty', () => {
    const result = filterBySupportedPlatformsAndAuthTypes(
      Platform.macOS,
      'local',
      [
        makeResourceSpec({
          name: 'Empty supportedPlatforms',
          supportedPlatforms: [],
        }),
        makeResourceSpec({
          name: 'Missing supportedPlatforms',
          supportedPlatforms: undefined,
        }),
        makeResourceSpec({
          name: 'Empty supportedAuthTypes',
          supportedAuthTypes: [],
        }),
        makeResourceSpec({
          name: 'Missing supportedAuthTypes',
          supportedAuthTypes: undefined,
        }),
      ]
    );

    expect(result).toHaveLength(4);
  });

  it('filters out resources based on supportedAuthTypes', () => {
    const ssoAndPasswordless = makeResourceSpec({
      name: 'Filtered out with many supported auth types',
      supportedAuthTypes: ['sso', 'passwordless'],
    });
    const sso = makeResourceSpec({
      name: 'Filtered out with one supported auth type',
      supportedAuthTypes: ['sso'],
    });
    const localAndPasswordless = makeResourceSpec({
      name: 'Kept with many supported auth types',
      supportedAuthTypes: ['local', 'passwordless'],
    });
    const local = makeResourceSpec({
      name: 'Kept with one supported auth type',
      supportedAuthTypes: ['local'],
    });

    const result = filterBySupportedPlatformsAndAuthTypes(
      Platform.macOS,
      'local',
      [ssoAndPasswordless, sso, localAndPasswordless, local]
    );

    expect(result).toContain(localAndPasswordless);
    expect(result).toContain(local);
    expect(result).not.toContain(ssoAndPasswordless);
    expect(result).not.toContain(sso);
  });
});
