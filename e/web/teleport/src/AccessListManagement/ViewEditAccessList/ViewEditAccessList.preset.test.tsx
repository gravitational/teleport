import { QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory } from 'history';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { Router } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  AccessListType,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import { ContextProvider } from 'teleport';
import { getAcl, noAccess } from 'teleport/mocks/contexts';
import {
  IntegrationStatusCode,
  type Plugin,
  type PluginOktaSpec,
} from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import ResourceService from 'teleport/services/resources';
import userService, { Acl } from 'teleport/services/user';

import {
  makeHandlers,
  unifiedResourcePath,
} from '../GuideEditor/Preset/TestHelper/mocks';
import {
  standardRoleWithDeny,
  testAccessListId,
} from '../GuideEditor/Preset/TestHelper/roles';
import { ViewEditAccessList } from './ViewEditAccessList';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(
    ...makeHandlers(),
    http.get(unifiedResourcePath, () => {
      return HttpResponse.json({
        items: [],
      });
    })
  );

  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(accessListWithPreset);

  jest
    .spyOn(accessManagementService, 'fetchAccessListsV2')
    .mockResolvedValue({ agents: [accessListWithPreset] });

  jest
    .spyOn(accessManagementService, 'fetchReviews')
    .mockResolvedValue({ reviews: [], startKey: '' });

  jest
    .spyOn(ResourceService.prototype, 'fetchRoles')
    .mockResolvedValue({ items: [], startKey: '' });

  jest
    .spyOn(ResourceService.prototype, 'fetchRolesV2')
    .mockResolvedValue({ items: [], startKey: '' });

  jest.spyOn(userService, 'fetchUsers').mockResolvedValue([]);

  jest
    .spyOn(userService, 'fetchUsersV2')
    .mockResolvedValue({ items: [], startKey: '' });

  jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue(oktaPlugin);
});

afterEach(async () => {
  jest.resetAllMocks();
  server.resetHandlers();
  await testQueryClient.resetQueries();
});

afterAll(() => server.close());

const getAccessDefinitionTab = () =>
  document.querySelector('[data-tab-id="tab-resource-access-definition"]');

describe('access definition tab visibility', () => {
  test('tab is visible when access list has a supported preset (long-term)', async () => {
    render(<Provider />);

    await screen.findByText(/mocked preset title/i);

    expect(getAccessDefinitionTab()).toBeInTheDocument();
  });

  test('tab is visible when access list has a supported preset (shot-term)', async () => {
    jest
      .spyOn(accessManagementService, 'fetchAccessList')
      .mockResolvedValue({ ...accessListWithPreset, preset: 'short-term' });

    render(<Provider />);

    await screen.findByText(/mocked preset title/i);

    expect(getAccessDefinitionTab()).toBeInTheDocument();
  });

  test('tab is NOT visible when access list has no preset', async () => {
    jest
      .spyOn(accessManagementService, 'fetchAccessList')
      .mockResolvedValue(accessListWithoutPreset);

    render(<Provider />);

    await screen.findByText(/mocked no preset title/i);

    expect(getAccessDefinitionTab()).not.toBeInTheDocument();
  });

  test('tab is NOT visible when user lacks read access (is only a member)', async () => {
    jest
      .spyOn(accessManagementService, 'fetchAccessList')
      .mockResolvedValue(accessListWithPresetViewingAsMember);

    render(<Provider customAcl={getAcl({ noAccess: true })} />);

    await screen.findByText(/mocked preset title/i);

    expect(getAccessDefinitionTab()).not.toBeInTheDocument();
  });

  test('tab is visible for a owner of access list despite not having access', async () => {
    jest
      .spyOn(accessManagementService, 'fetchAccessList')
      .mockResolvedValue(accessListWithPresetViewingAsOwner);

    render(<Provider customAcl={getAcl({ noAccess: true })} />);

    await screen.findByText(/mocked preset title/i);

    expect(getAccessDefinitionTab()).toBeInTheDocument();
  });
});

describe('access definition tab content', () => {
  test('shows error message when fetching roles fails', async () => {
    const user = userEvent.setup();

    jest
      .spyOn(ResourceService.prototype, 'fetchRolesV2')
      .mockRejectedValue(new Error('Failed to fetch roles'));

    render(<Provider />);

    await screen.findByText(/mocked preset title/i);
    await user.click(getAccessDefinitionTab()!);

    expect(
      await screen.findByText(/failed to fetch roles/i)
    ).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  test('shows permission error when user can view tab but lacks read role access', async () => {
    const user = userEvent.setup();

    render(
      <Provider
        customAcl={{
          ...getAcl(),
          roles: noAccess,
        }}
      />
    );

    await screen.findByText(/mocked preset title/i);
    await user.click(getAccessDefinitionTab()!);

    expect(
      await screen.findByText(
        /you do not have permission to read resource access/i
      )
    ).toBeInTheDocument();
  });
});

test('shows warning when role has unsupported fields (e.g. deny rules)', async () => {
  const user = userEvent.setup();

  jest.spyOn(ResourceService.prototype, 'fetchRolesV2').mockResolvedValue({
    items: [
      {
        name: standardRoleWithDeny.metadata.name,
        object: standardRoleWithDeny,
      } as any,
    ],
    startKey: '',
  });

  render(<Provider />);

  await screen.findByText(/mocked preset title/i);
  await user.click(getAccessDefinitionTab()!);

  expect(
    await screen.findByText(
      /the roles assigned to this access list cannot be parsed/i
    )
  ).toBeInTheDocument();

  expect(
    screen.getByRole('button', { name: /redefine access/i })
  ).toBeInTheDocument();

  expect(
    screen.getByRole('link', { name: /manually edit roles/i })
  ).toBeInTheDocument();
});

const Provider = ({ customAcl }: { customAcl?: Acl }) => {
  const history = createMemoryHistory();
  const ctx = createTeleportContextE({ customAcl });
  return (
    <Router history={history}>
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <ViewEditAccessList />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </QueryClientProvider>
    </Router>
  );
};

const oktaPlugin: Plugin<PluginOktaSpec, PluginStatusOkta> = {
  resourceType: 'plugin',
  kind: 'okta',
  name: 'plugin-name',
  statusCode: IntegrationStatusCode.Running,
};

const accessListWithPreset: AccessList = {
  id: testAccessListId,
  metadata: {
    name: testAccessListId,
    labels: {},
    revision: '',
  },
  type: AccessListType.Default,
  title: 'mocked preset title',
  description: 'some description',
  preset: 'long-term',
  owners: [
    {
      name: 'owner-1',
      description: '',
      ineligibleReason: '',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [
    {
      name: 'member-1',
      joined: new Date(),
      expires: new Date(),
      addedBy: 'llama',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  membersCount: 1,
  memberListCount: 0,
  grants: { roles: [''], traits: {} },
  ownerGrants: { roles: [''], traits: {} },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date(),
  },
  ownershipRequires: { roles: [], traits: {} },
  membershipRequires: { roles: [], traits: {} },
  inheritedMemberGrants: { roles: [], traits: {} },
};

const accessListWithoutPreset: AccessList = {
  ...accessListWithPreset,
  title: 'mocked no preset title',
  preset: '',
};

const accessListWithPresetViewingAsMember: AccessList = {
  ...accessListWithPreset,
  currentUserAssignments: {
    ownershipType: 0, // unspecified
    membershipType: 1, // explicit
  },
};

const accessListWithPresetViewingAsOwner: AccessList = {
  ...accessListWithPreset,
  currentUserAssignments: {
    ownershipType: 1, // explicit
    membershipType: 0, // unspecified
  },
};
