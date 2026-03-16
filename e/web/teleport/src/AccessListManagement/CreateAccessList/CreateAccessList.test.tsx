import { QueryClientProvider } from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import selectEvent from 'react-select-event';

import {
  act,
  enableMswServer,
  screen,
  server,
  testQueryClient,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import {
  AccessListMemberKind,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { InfoGuideSidePanel } from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';
import { UserContextProvider } from 'teleport/User';

import {
  AwsIcAppLabel,
  AwsIcRoleConditions,
} from '../GuideEditor/Preset/role/conditions/awsic';
import {
  defaultStandardRoleConditions,
  StandardRoleConditions,
} from '../GuideEditor/Preset/role/conditions/standard';
import { emptyRequiredAppIdentitiesWithFetchResult } from '../GuideEditor/Preset/role/resources/app';
import {
  fetchUnifiedResources,
  makeHandlers,
} from '../GuideEditor/Preset/TestHelper/mocks';
import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
import { CreateAccessList } from './CreateAccessList';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { ResumableCreateAccessListState } from './route';

const defaultIsEnterpriseFlag = cfg.oss.isEnterprise;
const defaultAccessListentitlement = cfg.oss.entitlements.AccessLists;

// URL paths with some query params stripped for msw path matching
const accessListPathV2 = cfg.api.accessListManagementPathV2.split('?')[0];
const usersPathV2 = cfg.oss.api.usersPathV2.split('?')[0];
const rolesPath = cfg.oss.api.role.list.split('?')[0];
const accessListPath = '/v1/enterprise/accesslist';

const accessListPresetPath = cfg.api.accessListPreset.create;
const mfaChallengePath = cfg.oss.api.mfaAuthnChallengePath;

// Raw API response format for access lists (before makeAccessList)
const rawAccessList = {
  spec: {
    type: '',
    title: 'All Employees',
    description: 'Adding new hires!',
    owners: [],
    grants: { roles: ['core-apps'], traits: {}, scoped_roles: [] },
    owner_grants: { roles: ['admin', 'root'], traits: {}, scoped_roles: [] },
    audit: {},
    ownership_requires: {},
    membership_requires: {},
  },
  metadata: {
    name: '1',
    labels: {},
    revision: '',
  },
  membersCount: 273,
};

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

mockIntersectionObserver();

enableMswServer();

beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
});

afterEach(async () => {
  jest.resetAllMocks();
  await testQueryClient.resetQueries();
});

describe('going through different guides', () => {
  const mockDate = new Date('2025-01-23T10:20:30Z');

  beforeEach(() => {
    jest.useFakeTimers();
    jest.setSystemTime(mockDate); // required for selecting date.

    cfg.oss.isEnterprise = true;
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    server.use(
      http.get(accessListPathV2, () => {
        return HttpResponse.json({
          accessLists: [rawAccessList],
          startKey: '',
        });
      }),
      http.get(usersPathV2, () => {
        return HttpResponse.json({
          items: [{ name: 'alice', roles: [] }],
          startKey: '',
        });
      }),
      http.get(rolesPath, () => {
        return HttpResponse.json({
          items: [
            { name: 'role-foo', id: 'role-id', kind: 'role', content: '' },
          ],
          startKey: '',
        });
      }),
      // Mock MFA challenge endpoint - return empty challenge (MFA not required)
      http.post(mfaChallengePath, () => {
        return HttpResponse.json({});
      })
    );
  });

  afterEach(() => {
    jest.resetAllMocks();
    jest.useRealTimers();

    cfg.oss.isEnterprise = defaultIsEnterpriseFlag;
    cfg.oss.entitlements.AccessLists = defaultAccessListentitlement;
  });

  test('custom', async () => {
    // Delay set to null here b/c using fake timers
    // (required for mock date and time) causes timeout on clicks.
    const user = userEvent.setup({ delay: null });

    let createAccessListCalled = false;
    let createAccessListRequest: unknown;
    server.use(
      http.post(accessListPath, async ({ request }) => {
        createAccessListCalled = true;
        createAccessListRequest = await request.json();
        return HttpResponse.json({ accessList: getRawAccessList() });
      })
    );

    const ctx = createTeleportContextE();
    renderComponent(ctx);

    await screen.findByText(/Select the type of Access List/i);
    await screen.findByText(/Page Info/i);

    // Click on custom tile
    await user.click(screen.getByText(/custom/i));
    await screen.findByText(/basic information/i);

    // Test validation prevents going forward.
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/title is required/i);

    // Fill out required inputs:

    // Basic
    await user.type(screen.getByPlaceholderText(/Title/i), 'some title');
    await user.click(screen.getByText(/Select a Date/i));
    await user.click(screen.queryAllByText(/26/)[0]);

    // Owners
    await selectEvent.select(
      screen.getByLabelText('Add Access Lists as Owners'),
      'All Employees'
    );
    await selectEvent.select(screen.getByLabelText('Add Owners'), 'alice');
    await screen.findByText(/alice/i);

    // Finish
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/some title successfully created/i);

    const expectedReq = getExpectedRequest();
    expectedReq.members = [];

    expect(createAccessListCalled).toBe(true);
    expect(createAccessListRequest).toEqual({
      ...expectedReq,
      audit: {
        ...expectedReq.audit,
        next_audit_date: expectedReq.audit.next_audit_date.toISOString(),
      },
    });
  });

  test('short-term with validation and error', async () => {
    // Delay set to null here b/c using fake timers
    // (required for mock date and time) causes timeout on clicks.
    const user = userEvent.setup({ delay: null });

    // First call returns error, subsequent calls succeed
    let createAccessListCalled = false;
    let createAccessListRequest: unknown;
    server.use(
      http.post(
        accessListPresetPath,
        () => {
          return HttpResponse.json(
            { error: { message: 'whoops error' } },
            { status: 500 }
          );
        },
        { once: true }
      ),
      http.post(accessListPresetPath, async ({ request }) => {
        createAccessListCalled = true;
        createAccessListRequest = await request.json();
        return HttpResponse.json({
          accessList: getRawAccessList(),
          members: [],
        });
      })
    );

    const ctx = createTeleportContextE();
    renderComponent(ctx);

    await screen.findByText(/Select the type of Access List/i);
    await screen.findByText(/Page Info/i);

    // Step 0: click on preset tile
    await user.click(screen.getByText(/temporary access/i));

    // Can skip step 1 & 2:
    await screen.findByText(/define access to resources/i);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/No resource access is defined/i);
    await user.click(screen.getAllByRole('button', { name: 'Next' })[1]);

    await screen.findByText(/define what identities/i);
    await user.click(screen.getByRole('button', { name: 'Next' }));

    // Step 3: fill out required basic info
    await screen.findByText(/step 3: basic information/i);

    // Test validation prevents going next.
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/title is required/i);

    await user.type(screen.getByPlaceholderText(/Title/i), 'some title');
    await user.click(screen.getByText(/select a date/i));
    await user.click(screen.queryAllByText(/26/)[0]);

    await user.click(screen.getByText(/next/i));
    await screen.findByText(/who should be required to request access/i);

    // Step 4: fill out required membership

    // Test validation prevents going next.
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/please select at least one user/i);

    expect(
      screen.getByText(/made required to request for access to resources/i)
    ).toBeInTheDocument();

    await selectEvent.select(
      screen.getByLabelText('Add Access Lists as Members'),
      'All Employees'
    );
    await screen.findByText(/All Employees/i);

    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/who should review access requests/i);

    // Step 5: fill out required owners

    // Test validation prevents going next.
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/please select at least one user/i);

    await selectEvent.select(
      screen.getByLabelText('Add Access Lists as Owners'),
      'All Employees'
    );
    await screen.findByText(/All Employees/i);

    // act required otherwise it complains of:
    // An update to ForwardRef inside a test was not wrapped in act
    await act(async () => {
      await selectEvent.select(screen.getByLabelText('Owner Type'), 'Users');
    });
    await selectEvent.select(screen.getByLabelText('Add Owners'), 'alice');

    // Step 6: finished

    // Test error renders dialogue
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/whoops error/i);

    // Try again should succeed
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/some title successfully created/i);

    const { members: expectedMembers, ...expectedSpec } = getExpectedRequest();
    expect(createAccessListCalled).toBe(true);

    expect(createAccessListRequest).toEqual({
      presetType: 'short-term',
      accessList: {
        spec: {
          ...expectedSpec,
          audit: {
            ...expectedSpec.audit,
            next_audit_date: expectedSpec.audit.next_audit_date.toISOString(),
          },
        },
        members: expectedMembers.map(m => ({
          ...m,
          joined: expect.any(String),
        })),
        metadata: expect.objectContaining({ labels: {}, revision: '' }),
      },
      accessRoles: [],
    });
  });

  test('long-term with optional advanced settings', async () => {
    // Delay set to null here b/c using fake timers
    // (required for mock date and time) causes timeout on clicks.
    const user = userEvent.setup({ delay: null });

    // First call returns error, subsequent calls succeed
    let createAccessListCalled = false;
    let createAccessListRequest: unknown;
    server.use(
      http.post(
        accessListPresetPath,
        () => {
          return HttpResponse.json(
            { error: { message: 'whoops error' } },
            { status: 500 }
          );
        },
        { once: true }
      ),
      http.post(accessListPresetPath, async ({ request }) => {
        createAccessListCalled = true;
        createAccessListRequest = await request.json();
        return HttpResponse.json({
          accessList: getRawAccessList(),
          members: [],
        });
      })
    );

    const ctx = createTeleportContextE();
    renderComponent(ctx);

    await screen.findByText(/Select the type of Access List/i);
    await screen.findByText(/Page Info/i);

    // Step 0: click on preset tile
    await user.click(screen.getByText(/long-lived access/i));

    // Can skip step 1 & 2:
    await screen.findByText(/define access to resources/i);
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/No resource access is defined/i);
    await user.click(screen.getAllByRole('button', { name: 'Next' })[1]);

    await screen.findByText(/define what identities/i);
    await user.click(screen.getByRole('button', { name: 'Next' }));

    // Step 3: fill out required basic info
    await screen.findByText(/step 3: basic information/i);

    await user.type(screen.getByPlaceholderText(/Title/i), 'some title');
    await user.click(screen.getByText(/select a date/i));
    await user.click(screen.queryAllByText(/26/)[0]);

    // Step 4: fill out required membership
    await user.click(screen.getByText(/next/i));
    await screen.findByText(/who are you setting up access for/i);
    expect(screen.getByText(/will be given access/i)).toBeInTheDocument();

    await selectEvent.select(
      screen.getByLabelText('Add Access Lists as Members'),
      'All Employees'
    );
    await screen.findByText(/All Employees/i);

    await user.click(screen.getByText(/optional advanced settings/i));
    await screen.findByText(/membership will have no effect/i);
    await selectEvent.select(
      screen.getByLabelText(/required roles/i),
      'role-foo'
    );
    await screen.findByText(/role-foo/i);

    // Step 5: fill out required owners
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(
      /who should periodically review and audit memberships/i
    );

    await selectEvent.select(
      screen.getByLabelText('Add Access Lists as Owners'),
      'All Employees'
    );
    await screen.findByText(/All Employees/i);

    // act required otherwise it complains of:
    // An update to ForwardRef inside a test was not wrapped in act
    await act(async () => {
      await selectEvent.select(screen.getByLabelText('Owner Type'), 'Users');
    });
    await selectEvent.select(screen.getByLabelText('Add Owners'), 'alice');

    await user.click(screen.getByText(/optional advanced settings/i));
    await screen.findByText(/ownership will have no effect/i);
    await selectEvent.select(
      screen.getByLabelText(/required roles/i),
      'role-foo'
    );
    await screen.findByText(/role-foo/i);

    // Step 6: finished

    // Test error renders dialogue
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/whoops error/i);

    // Try again should succeed
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/some title successfully created/i);

    const { members: expectedMembers, ...expectedSpec } = getExpectedRequest();
    expectedSpec.membership_requires.roles = ['role-foo'];
    expectedSpec.ownership_requires.roles = ['role-foo'];
    expect(createAccessListCalled).toBe(true);

    expect(createAccessListRequest).toEqual({
      presetType: 'long-term',
      accessList: {
        spec: {
          ...expectedSpec,
          audit: {
            ...expectedSpec.audit,
            next_audit_date: expectedSpec.audit.next_audit_date.toISOString(),
          },
        },
        members: expectedMembers.map(m => ({
          ...m,
          joined: expect.any(String),
        })),
        metadata: expect.objectContaining({ labels: {}, revision: '' }),
      },
      accessRoles: [],
    });
  });

  test('resumability from last step via URL location state', async () => {
    const user = userEvent.setup({ delay: null });

    let createAccessListRequest: unknown;
    const createdAccessListId = 'created-access-list-id';
    server.use(
      http.post(accessListPresetPath, async ({ request }) => {
        createAccessListRequest = await request.json();
        return HttpResponse.json({
          accessList: {
            spec: {
              ...getExpectedRequest(),
              title: 'Resumed Access List',
              description: 'Testing resumability',
            },
            metadata: { name: createdAccessListId },
          },
          members: [],
        });
      })
    );

    const ctx = createTeleportContextE();

    const resumableStandardConditions: StandardRoleConditions = {
      ...defaultStandardRoleConditions(),
      app_labels: { env: 'prod', team: 'engineering' },
      db_labels: { tier: 'primary' },
      node_labels: { region: 'us-west-2' },
      aws_role_arns: ['arn:aws:iam::123456789:role/admin'],
      azure_identities: ['/subscriptions/sub-id/providers/Microsoft.Compute'],
      logins: ['root', 'ubuntu'],
      db_names: ['postgres', 'mysql'],
      db_users: ['admin'],
    };

    const resumableAwsIcDontidions: AwsIcRoleConditions = {
      labels: AwsIcAppLabel,
      account: new Map([
        ['111122223333', new Set(['arn:aws:sso:::permissionSet/ps-admin'])],
        [
          '444455556666',
          new Set([
            'arn:aws:sso:::permissionSet/ps-readonly',
            'arn:aws:sso:::permissionSet/ps-developer',
          ]),
        ],
      ]),
    };

    const resumableState: ResumableCreateAccessListState = {
      oktaOrgUrl: '',
      preset: 'short-term',
      resumeStep: 4,
      standardRoleConditions: resumableStandardConditions,
      requiredAppIdentities: emptyRequiredAppIdentitiesWithFetchResult(),
      awsIcRoleConditions: resumableAwsIcDontidions,

      spec: {
        title: 'Resumed Access List',
        description: 'Testing resumability',
        reviewDayOfMonth: reviewDayOfMonthOpts.find(
          o => o.value === ReviewDayOfMonth.FifteenthDayOfMonth
        ),
        reviewFrequency: reviewFrequencyOpts.find(
          o => o.value === ReviewFrequency.OneYear
        ),
        auditStartDate: new Date('2025-01-20T00:00:00.000Z'),
      },
      owners: {
        selectedRolesRequired: [{ value: 'admin', label: 'admin' }],
        eligibleOwners: [],
        selectedOwners: [
          {
            value: {
              name: 'bob',
              membershipKind: AccessListMemberKind.User,
            },
            label: 'bob',
          },
        ],
        traitLabels: [],
        traitLookup: {},
      },
      ownerGrant: {
        rolesToGrant: [],
        traitsToGrant: [],
      },
      members: {
        selectedRolesRequired: [{ value: 'developer', label: 'developer' }],
        eligibleMembers: [],
        selectedMembers: [
          {
            value: {
              name: '1',
              membershipKind: AccessListMemberKind.List,
            },
            label: 'All Employees',
          },
        ],
        traitLabels: [],
        traitLookup: {},
      },
      memberGrant: {
        rolesToGrant: [],
        traitsToGrant: [],
      },
    };

    renderComponent(ctx, resumableState);

    // Should start at Define Ownership step
    await screen.findByText(/who should review access requests/i);

    // Complete the flow
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/Resumed Access List successfully created/i);

    // Verify the request contains the resumed state data including role conditions
    expect(createAccessListRequest).toEqual(
      expect.objectContaining({
        presetType: 'short-term',
        accessList: expect.objectContaining({
          spec: expect.objectContaining({
            title: 'Resumed Access List',
            description: 'Testing resumability',
            audit: expect.objectContaining({
              next_audit_date: '2025-01-20T00:00:00.000Z',
              recurrence: expect.objectContaining({
                day_of_month: '15',
                frequency: '12m',
              }),
            }),
            ownership_requires: expect.objectContaining({
              roles: ['admin'],
            }),
            owners: expect.arrayContaining([
              expect.objectContaining({
                name: 'bob',
                membership_kind: 'MEMBERSHIP_KIND_USER',
              }),
            ]),
            membership_requires: expect.objectContaining({
              roles: ['developer'],
            }),
          }),
          members: expect.arrayContaining([
            expect.objectContaining({
              name: '1',
              membership_kind: 'MEMBERSHIP_KIND_LIST',
            }),
          ]),
        }),
        accessRoles: expect.arrayContaining([
          expect.objectContaining({
            metadata: expect.objectContaining({ name: 'access-standard' }),
            spec: expect.objectContaining({
              allow: expect.objectContaining(resumableStandardConditions),
            }),
          }),
          expect.objectContaining({
            metadata: expect.objectContaining({ name: 'access-awsic' }),
            spec: expect.objectContaining({
              allow: expect.objectContaining({
                app_labels: AwsIcAppLabel,
                account_assignments: expect.arrayContaining([
                  {
                    account: '111122223333',
                    permission_set: 'arn:aws:sso:::permissionSet/ps-admin',
                  },
                  {
                    account: '444455556666',
                    permission_set: 'arn:aws:sso:::permissionSet/ps-readonly',
                  },
                  {
                    account: '444455556666',
                    permission_set: 'arn:aws:sso:::permissionSet/ps-developer',
                  },
                ]),
              }),
            }),
          }),
        ]),
      })
    );
  });
});

function renderComponent(
  ctx: TeleportEContext,
  locationState?: ResumableCreateAccessListState
) {
  return renderWithMemoryRouter(
    <QueryClientProvider client={testQueryClient}>
      <InfoGuidePanelProvider>
        <AccessGraphDemoProvider>
          <UserContextProvider>
            <ContextProvider ctx={ctx}>
              <AccessListManagementContextProvider>
                <CreateAccessListContextProvider>
                  <CreateAccessList />
                </CreateAccessListContextProvider>
              </AccessListManagementContextProvider>
            </ContextProvider>
          </UserContextProvider>
        </AccessGraphDemoProvider>
        <InfoGuideSidePanel />
      </InfoGuidePanelProvider>
    </QueryClientProvider>,
    {
      initialEntries: [
        { pathname: cfg.routes.accessListNew, state: locationState },
      ],
    }
  );
}

function getRawAccessList() {
  return {
    spec: { ...getExpectedRequest() },
    metadata: { name: 'some-id' },
  };
}

function getExpectedRequest() {
  return {
    type: '',
    audit: {
      next_audit_date: new Date('2025-01-26T00:00:00.000Z'),
      recurrence: {
        day_of_month: '1',
        frequency: '6m',
      },
    },
    description: '',
    grants: {
      roles: [],
      scoped_roles: [],
      traits: {},
    },
    members: [
      {
        added_by: 'llama',
        joined: new Date('2025-01-23T10:20:30.000Z'),
        membership_kind: 'MEMBERSHIP_KIND_LIST',
        name: '1',
      },
    ],
    membership_requires: {
      roles: [],
      traits: {},
    },
    owner_grants: {
      roles: [],
      scoped_roles: [],
      traits: {},
    },
    owners: [
      {
        membership_kind: 'MEMBERSHIP_KIND_LIST',
        name: '1',
      },
      {
        membership_kind: 'MEMBERSHIP_KIND_USER',
        name: 'alice',
      },
    ],
    ownership_requires: {
      roles: [],
      traits: {},
    },
    title: 'some title',
  };
}
