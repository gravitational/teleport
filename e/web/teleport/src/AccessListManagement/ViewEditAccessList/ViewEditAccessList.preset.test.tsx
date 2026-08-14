import { QueryClientProvider } from '@tanstack/react-query';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter, RouterProvider } from 'react-router';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
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
import { UserContextProvider } from 'teleport/User';

import { AwsIcAppLabel } from '../GuideEditor/Preset/role/conditions';
import {
  appsWithAllMatchingPermissionSet,
  fetchUnifiedResources,
  makeHandlers,
  unifiedResourcePath,
} from '../GuideEditor/Preset/TestHelper/mocks';
import {
  awsIcRoleEmptyAccess,
  standardRoleWithDeny,
  testAccessListId,
} from '../GuideEditor/Preset/TestHelper/roles';
import { ViewEditAccessList } from './ViewEditAccessList';

enableMswServer();
mockIntersectionObserver();

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

  jest
    .spyOn(userService, 'fetchUsersV2')
    .mockResolvedValue({ items: [], startKey: '' });

  jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue(oktaPlugin);
});

afterEach(async () => {
  jest.resetAllMocks();
  await testQueryClient.resetQueries();
});

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

test('preserves AwsIcAppLabel when re-adding access to an existing aws ic role with empty allow', async () => {
  const user = userEvent.setup();

  const accessListWithAwsIc: AccessList = {
    ...accessListWithPreset,
    grants: {
      roles: [awsIcRoleEmptyAccess.metadata.name],
      traits: {},
      scopedRoles: [],
    },
  };

  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(accessListWithAwsIc);

  jest.spyOn(ResourceService.prototype, 'fetchRolesV2').mockResolvedValue({
    items: [
      {
        name: awsIcRoleEmptyAccess.metadata.name,
        object: awsIcRoleEmptyAccess,
      } as any,
    ],
    startKey: '',
  });

  server.use(
    ...makeHandlers([
      fetchUnifiedResources('get', appsWithAllMatchingPermissionSet),
    ])
  );

  let updateRequest: any;
  jest
    .spyOn(accessManagementService, 'updateAccessListWithPreset')
    .mockImplementation(async req => {
      updateRequest = req;
      return { accessList: accessListWithAwsIc, rolesToBeDeleted: [] };
    });

  render(<Provider />);

  await screen.findByText(/mocked preset title/i);
  await user.click(getAccessDefinitionTab()!);

  // Open editor.
  await screen.findByText(/define resource access/i);
  await user.click(
    screen.getByRole('button', { name: /define resource access/i })
  );

  // Switch to AWS IC tab and add an account + permission set.
  await user.click(await screen.findByTestId('awsIc'));
  await user.click(
    await screen.findByRole('button', { name: /make a new selection/i })
  );
  await user.click(
    screen.getByRole('checkbox', { name: /app-friendly-name-1/i })
  );
  await user.click(screen.getByRole('checkbox', { name: /ps-name-1/i }));
  await user.click(screen.getByRole('button', { name: /add selection/i }));

  // Save changes.
  await user.click(screen.getByRole('button', { name: /^next$/i }));
  await user.click(
    await screen.findByRole('button', { name: /save changes/i })
  );

  await waitFor(() => expect(updateRequest).toBeDefined());

  expect(updateRequest.accessRoles).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        metadata: expect.objectContaining({
          name: awsIcRoleEmptyAccess.metadata.name,
        }),
        spec: expect.objectContaining({
          allow: expect.objectContaining({
            app_labels: AwsIcAppLabel,
          }),
        }),
      }),
    ])
  );
});

test('shows warning when role has unsupported fields (e.g. deny rules)', async () => {
  const user = userEvent.setup();

  const accessListWithDenyRole: AccessList = {
    ...accessListWithPreset,
    grants: {
      roles: [standardRoleWithDeny.metadata.name],
      traits: {},
      scopedRoles: [],
    },
  };

  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(accessListWithDenyRole);

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
      /the member roles assigned to this access list cannot be parsed/i
    )
  ).toBeInTheDocument();

  expect(screen.getByText(/manually edit member access/i)).toBeInTheDocument();

  expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
});

const Provider = ({ customAcl }: { customAcl?: Acl }) => {
  const ctx = createTeleportContextE({ customAcl });
  const router = createMemoryRouter([
    {
      path: '*',
      element: (
        <QueryClientProvider client={testQueryClient}>
          <InfoGuidePanelProvider>
            <UserContextProvider>
              <ContextProvider ctx={ctx}>
                <AccessGraphDemoProvider>
                  <AccessListManagementContextProvider>
                    <ViewEditAccessList />
                  </AccessListManagementContextProvider>
                </AccessGraphDemoProvider>
              </ContextProvider>
            </UserContextProvider>
          </InfoGuidePanelProvider>
        </QueryClientProvider>
      ),
    },
  ]);

  return <RouterProvider router={router} />;
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
  grants: { roles: [''], traits: {}, scopedRoles: [] },
  ownerGrants: { roles: [''], traits: {}, scopedRoles: [] },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date(),
  },
  ownershipRequires: { roles: [], traits: {} },
  membershipRequires: { roles: [], traits: {} },
  inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
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
