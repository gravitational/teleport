import { QueryClientProvider } from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

import { act, render, screen, testQueryClient } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { mockAccessLists } from 'e-teleport/AccessListManagement/AccessLists/EmptyState/fixtures';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { makeAccessList } from 'e-teleport/services/accessmanagement/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { InfoGuideSidePanel } from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';
import cfg from 'teleport/config';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';

import { unifiedResourcePath } from '../GuideEditor/Preset/TestHelper/mocks';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { CreateAccessList } from './CreateAccessListV2';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListentitlement = cfg.entitlements.AccessLists;

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

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
});

afterEach(async () => {
  jest.resetAllMocks();
  server.resetHandlers();
  await testQueryClient.resetQueries();
});

afterAll(() => server.close());

describe('going through different guides', () => {
  // Delay set to null here b/c using fake timers
  // (required for mock date and time) causes timeout on clicks.
  const user = userEvent.setup({ delay: null });
  const mockDate = new Date('2025-01-23T10:20:30Z');

  beforeEach(() => {
    jest.useFakeTimers();
    jest.setSystemTime(mockDate);

    cfg.isEnterprise = true;
    cfg.entitlements.AccessLists = { enabled: true, limit: 0 };

    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [mockAccessLists[0]] });
    jest.spyOn(userService, 'fetchUsersV2').mockResolvedValue({
      startKey: '',
      items: [{ name: 'alice', roles: [] }],
    });
    jest.spyOn(ResourceService.prototype, 'fetchRoles').mockResolvedValue({
      items: [{ name: 'role-foo', id: 'role-id', kind: 'role', content: '' }],
      startKey: '',
    });
    jest
      .spyOn(pluginsService, 'fetchPlugin')
      .mockResolvedValue({} as Plugin<PluginOktaSpec, PluginStatusOkta>);
  });

  afterEach(() => {
    jest.resetAllMocks();
    jest.useRealTimers();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.entitlements.AccessLists = defaultAccessListentitlement;
  });

  test('custom', async () => {
    const spiedCreateAccessList = jest
      .spyOn(accessManagementService, 'createAccessList')
      .mockResolvedValue(testAccessList);

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

    expect(spiedCreateAccessList).toHaveBeenCalledWith(expectedReq);
  });

  test('short-term with validation', async () => {
    const spiedCreateAccessList = jest
      .spyOn(accessManagementService, 'createAccessList')
      .mockRejectedValueOnce(new Error('whoops error'))
      .mockResolvedValue(testAccessList);

    const ctx = createTeleportContextE();
    renderComponent(ctx);

    await screen.findByText(/Select the type of Access List/i);
    await screen.findByText(/Page Info/i);

    // Step 0: click on preset tile
    await user.click(screen.getByText(/grants members temporary access/i));
    await screen.findByText(/step 1: basic information/i);

    // Step 1: fill out required basic info

    // Test validation prevents going next.
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/title is required/i);

    await user.type(screen.getByPlaceholderText(/Title/i), 'some title');
    await user.click(screen.getByText(/select a date/i));
    await user.click(screen.queryAllByText(/26/)[0]);

    await user.click(screen.getByText(/next/i));
    await screen.findByText(/who should be required to request access/i);

    // Step 2: fill out required membership

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

    // Step 3: fill out required owners

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

    // Step 4: finished

    // Test error renders dialogue
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/whoops error/i);

    // Try again should succeed
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/some title successfully created/i);

    expect(spiedCreateAccessList).toHaveBeenCalledWith(getExpectedRequest());
  });

  test('long-term with optional advanced settings', async () => {
    const spiedCreateAccessList = jest
      .spyOn(accessManagementService, 'createAccessList')
      .mockRejectedValueOnce(new Error('whoops error'))
      .mockResolvedValue(testAccessList);

    const ctx = createTeleportContextE();
    renderComponent(ctx);

    await screen.findByText(/Select the type of Access List/i);
    await screen.findByText(/Page Info/i);

    // Step 0: click on preset tile
    await user.click(screen.getByText(/long-lived access/i));
    await screen.findByText(/step 1: basic information/i);

    // Step 1: fill out required basic info
    await user.type(screen.getByPlaceholderText(/Title/i), 'some title');
    await user.click(screen.getByText(/select a date/i));
    await user.click(screen.queryAllByText(/26/)[0]);

    // Step 2: fill out required membership
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

    // Step 3: fill out required owners
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

    // Step 4: finished

    // Test error renders dialogue
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/whoops error/i);

    // Try again should succeed
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText(/some title successfully created/i);

    const expectedReq = getExpectedRequest();
    expectedReq.membership_requires.roles = ['role-foo'];
    expectedReq.ownership_requires.roles = ['role-foo'];
    expect(spiedCreateAccessList).toHaveBeenCalledWith(expectedReq);
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={testQueryClient}>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <AccessListManagementContextProvider>
              <CreateAccessListContextProvider>
                <CreateAccessList />
              </CreateAccessListContextProvider>
            </AccessListManagementContextProvider>
          </ContextProvider>
          <InfoGuideSidePanel />
        </InfoGuidePanelProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
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

const testAccessList = makeAccessList({
  spec: { ...getExpectedRequest() },
  metadata: { name: 'some-id' },
});
