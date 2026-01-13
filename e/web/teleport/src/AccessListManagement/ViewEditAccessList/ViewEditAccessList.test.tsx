import { QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, History } from 'history';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { Router } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  within,
} from 'design/utils/testing';
import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  AccessListOrigin,
  AccessListReview,
  AccessListType,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';
import {
  IntegrationStatusCode,
  type Plugin,
  type PluginOktaSpec,
} from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import ResourceService from 'teleport/services/resources';
import userService, { Acl } from 'teleport/services/user';

import { unifiedResourcePath } from '../GuideEditor/Preset/testHelper';
import { ViewEditAccessList } from './ViewEditAccessList';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(
    http.get(unifiedResourcePath, () => {
      return HttpResponse.json({
        items: [],
      });
    })
  );

  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(accessList);

  jest
    .spyOn(accessManagementService, 'fetchAccessListsV2')
    .mockResolvedValue({ agents: [accessList] });
  jest
    .spyOn(accessManagementService, 'fetchReviews')
    .mockResolvedValue({ reviews, startKey: '' });
  jest
    .spyOn(ResourceService.prototype, 'fetchRoles')
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

test('back button uses previous route if present and preserves queries', async () => {
  const history = createMemoryHistory({
    initialEntries: [`${cfg.getAccessListManagementRoute()}?search=banana`],
  });
  history.goBack = jest.fn();

  render(<Provider customHistory={history} />);

  await screen.findByText(/mocked title/i);
  await userEvent.click(screen.getByTestId('back-button'));
  expect(history.goBack).toHaveBeenCalled();
  expect(history.location?.pathname).toBe(cfg.getAccessListManagementRoute());
  expect(history.location?.search).toBe('?search=banana');
});

test('back button uses default route if router state is not provided', async () => {
  const history = createMemoryHistory();
  history.push = jest.fn();
  // Manually unset location.key to simulate initial page load in-browser
  history.location.key = undefined;

  render(<Provider customHistory={history} />);

  await screen.findByText(/mocked title/i);
  await userEvent.click(screen.getByTestId('back-button'));
  expect(history.push).toHaveBeenCalledWith(cfg.getAccessListManagementRoute());
});

test('viewing access list as admin (has access list rbac)', async () => {
  render(<Provider />);

  await screen.findByText(/mocked title/i);
  expect(screen.getByText(/some description/i)).toBeInTheDocument();

  await testEditAccess({ as: 'admin' });
});

test('viewing access list as owner (no rbac)', async () => {
  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(viewingAsOwner);

  render(<Provider customAcl={getAcl({ noAccess: true })} />);

  await screen.findByText(/mocked title/i);
  expect(screen.getByText(/some description/i)).toBeInTheDocument();

  await testEditAccess({ as: 'owner-no-rbac' });
});

test('viewing access list as member (no rbac)', async () => {
  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(viewingAsMember);

  render(<Provider customAcl={getAcl({ noAccess: true })} />);

  await screen.findByText(/mocked title/i);
  expect(screen.getByText(/some description/i)).toBeInTheDocument();

  await testEditAccess({ as: 'member' });
});

test('viewing an okta access list as admin (has access list rbac)', async () => {
  jest
    .spyOn(pluginsService, 'fetchPlugin')
    .mockResolvedValue(oktaPluginWithBidirectionalSync);

  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(viewingOktaDerivedList);

  render(<Provider />);

  await screen.findByText(/mocked title/i);
  expect(screen.getByText(/some description/i)).toBeInTheDocument();

  await testEditAccess({ as: 'admin-okta-bidirectionalsync' });
});

type TestAs =
  | 'admin'
  | 'owner-no-rbac'
  | 'member'
  | 'admin-okta-bidirectionalsync';

async function testEditAccess({ as }: { as: TestAs }) {
  await testAccessToEditHeaderContents({ as });
  await testAccessToEditMembersContent({ as });
  await testAccessToEditOwnersContent({ as });
  await testAccessToEditAudit({ as });
}

async function testAccessToEditMembersContent({ as }: { as: TestAs }) {
  expect(screen.getByText(/members \(1\)/i)).toBeInTheDocument();
  // edit requirements
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(screen.getByTestId('btn-member-requirements'));
    expect(screen.getByText(/edit member eligibility/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-member-requirements')).toBeDisabled();
  }

  // edit perm grants
  if (as === 'admin') {
    await userEvent.click(screen.getByTestId('btn-member-grants'));
    expect(
      screen.getByText(/edit member permissions granted/i)
    ).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-member-grants')).toBeDisabled();
  }

  // add new members
  if (
    as === 'admin' ||
    as === 'admin-okta-bidirectionalsync' ||
    as === 'owner-no-rbac'
  ) {
    await userEvent.click(screen.getByText(/add new members/i));
    expect(screen.getByText(/reason for enrolling/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    // a member can't see other members
    expect(screen.queryByText(/add new members/i)).not.toBeInTheDocument();
  }

  // delete members
  const withinMembersContent = within(screen.getByTestId('members-content'));
  if (
    as === 'admin' ||
    as === 'admin-okta-bidirectionalsync' ||
    as === 'owner-no-rbac'
  ) {
    await userEvent.click(withinMembersContent.getByText(/delete/i));
    expect(screen.getByText(/delete member\?/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(withinMembersContent.queryByText(/delete/i)).not.toBeInTheDocument();
  }
}

async function testAccessToEditOwnersContent({ as }: { as: TestAs }) {
  await userEvent.click(screen.getByText(/owners \(1\)/i));

  // edit requirements
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(screen.getByTestId('btn-owner-requirements'));
    expect(screen.getByText(/edit owner eligibility/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-owner-requirements')).toBeDisabled();
  }

  // edit perm grants
  if (as === 'admin') {
    await userEvent.click(screen.getByTestId('btn-owner-grants'));
    expect(
      screen.getByText(/edit owner permissions granted/i)
    ).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-owner-grants')).toBeDisabled();
  }

  // add new owners
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(screen.getByText(/add new owners/i));
    expect(screen.getByText(/describe owner/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByText(/add new owners/i)).toBeDisabled();
  }

  // delete owners
  const withinOwnersContent = within(screen.getByTestId('owners-content'));
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(withinOwnersContent.getByText(/delete/i));
    expect(screen.getByText(/delete owner\?/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(withinOwnersContent.getByText(/delete/i)).toBeDisabled();
  }
}

async function testAccessToEditAudit({ as }: { as: TestAs }) {
  await userEvent.click(screen.getByText(/audits/i));

  // edit audit
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(screen.getByTestId('btn-audit'));
    expect(screen.getByText(/save audit/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-audit')).toBeDisabled();
  }

  // view audits
  if (
    as === 'admin' ||
    as === 'admin-okta-bidirectionalsync' ||
    as === 'owner-no-rbac'
  ) {
    await userEvent.click(screen.getByText(/view changes/i));
    await userEvent.click(screen.getByText('Review Changes'));
    await userEvent.click(screen.getByText(/close/i));
  } else {
    expect(screen.queryByText('Past Audits')).not.toBeInTheDocument();
  }
}

async function testAccessToEditHeaderContents({ as }: { as: TestAs }) {
  // can user review list
  if (
    as === 'admin' ||
    as === 'admin-okta-bidirectionalsync' ||
    as === 'owner-no-rbac'
  ) {
    expect(
      screen.getByText(/this access list requires review/i)
    ).toBeInTheDocument();
  } else {
    expect(
      screen.queryByText(/this access list requires review/i)
    ).not.toBeInTheDocument();
  }

  // can user edit title
  if (as === 'admin') {
    await userEvent.click(screen.getByTestId('btn-title'));
    const targetText = screen.queryAllByText(/edit title/i);
    expect(targetText.length).toBeGreaterThan(0);
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-title')).toBeDisabled();
  }

  // can user edit description
  if (as === 'admin') {
    await userEvent.click(screen.getByTestId('btn-description'));
    const targetText = screen.queryAllByText(/edit description/i);
    expect(targetText.length).toBeGreaterThan(0);
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(screen.getByTestId('btn-description')).toBeDisabled();
  }

  // can user delete
  const withinHeader = within(screen.getByTestId('header'));
  if (as === 'admin' || as === 'admin-okta-bidirectionalsync') {
    await userEvent.click(withinHeader.getByText(/delete/i));
    expect(screen.getByText(/delete access list\?/i)).toBeInTheDocument();
    await userEvent.click(screen.getByText(/cancel/i));
  } else {
    expect(withinHeader.getByText(/delete/i)).toBeDisabled();
  }

  // title badge
  if (as === 'admin-okta-bidirectionalsync') {
    expect(
      screen.getByTestId(`badge-${AccessListOrigin.Okta}`)
    ).toBeInTheDocument();
  } else {
    expect(screen.queryByTestId(/badge-*/i)).not.toBeInTheDocument();
  }
}

const Provider = ({
  customAcl,
  customHistory = createMemoryHistory(),
}: {
  customAcl?: Acl;
  customHistory?: History;
}) => {
  const ctx = createTeleportContextE({ customAcl });
  return (
    <Router history={customHistory}>
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

const oktaPluginWithBidirectionalSync: Plugin<
  PluginOktaSpec,
  PluginStatusOkta
> = {
  ...oktaPlugin,
  spec: {
    scimBearerToken: '',
    oktaAppId: '',
    oktaAppName: '',
    teleportSsoConnector: '',
    error: '',
    defaultOwners: [],
    orgUrl: '',
    enableBidirectionalSync: true,
  },
};

const reviews: AccessListReview[] = [
  {
    notes: 'some-note',
    reviewDate: new Date(),
    reviewers: ['lisa'],
    raw: {
      kind: 'access_list_review',
      version: 'v1',
      metadata: {
        name: '147879cd-5d78-4c10-a215-c6339dfada72',
        expires: '0001-01-01T00:00:00Z',
        revision: '8ec832a4-6960-479a-9230',
      },
      spec: {
        review_date: '2025-06-26T23:09:50.604529Z',
        access_list: '270a3348-3711-421e-a053-6914da6aeb46',
        reviewers: ['lisa'],
        notes: '',
        changes: {
          review_frequency_changed: '',
          review_day_of_month_changed: '',
          membership_requirements_changed: null,
          removed_members: null,
        },
      },
    },
  },
];

const accessList: AccessList = {
  id: 'some-id-123',
  type: AccessListType.Default,
  title: 'mocked title',
  description: 'some description',
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
  membersCount: 0,
  memberListCount: 0,
  grants: { roles: [''], traits: {} }, // member grant
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

const viewingAsOwner: AccessList = {
  ...accessList,
  currentUserAssignments: {
    ownershipType: AccessListUserAssignmentType.EXPLICIT,
    membershipType: AccessListUserAssignmentType.UNSPECIFIED,
  },
};

const viewingAsMember: AccessList = {
  ...accessList,
  currentUserAssignments: {
    ownershipType: AccessListUserAssignmentType.UNSPECIFIED,
    membershipType: AccessListUserAssignmentType.EXPLICIT,
  },
};

const viewingOktaDerivedList: AccessList = {
  ...accessList,
  origin: AccessListOrigin.Okta,
};
