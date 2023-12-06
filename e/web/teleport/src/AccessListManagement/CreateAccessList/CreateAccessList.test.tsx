import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { getAcl } from 'teleport/mocks/contexts';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';

import {
  getEligibleUsers,
  getEligibleUsersAmongSelectedUsers,
  CreateAccessList,
} from './CreateAccessList';

const defaultIsTeamFlag = cfg.isTeam;
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultIgsFlag = cfg.isIgsEnabled;
const defaultCreateLimit = cfg.featureLimits.accessListCreateLimit;
const defaultIsCloud = cfg.isCloud;

describe('upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;
    cfg.isCloud = true;

    // Response doesn't matter, just that we have one element in array.
    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([{} as any]);

    jest.spyOn(userService, 'fetchUsers').mockResolvedValue([]);
    jest.spyOn(ResourceService.prototype, 'fetchRoles').mockResolvedValue([]);
  });

  afterEach(() => {
    jest.resetAllMocks();

    cfg.isTeam = defaultIsTeamFlag;
    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.isIgsEnabled = defaultIgsFlag;
    cfg.featureLimits.accessListCreateLimit = defaultCreateLimit;
    cfg.isCloud = defaultIsCloud;
  });

  test('no access should not render cta', async () => {
    ecfg.oss.isIgsEnabled = true;
    ecfg.oss.isTeam = false;

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);

    await screen.findByText(
      /Only Teleport administrators can create new Access Lists/i
    );
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('render limited preview for non-cloud', async () => {
    ecfg.oss.isCloud = false;

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);

    await screen.findByText(/preview and will limit/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('eub with igs enabled renders no cta', async () => {
    ecfg.oss.isIgsEnabled = true;
    ecfg.oss.isTeam = false;

    renderComponent(ctx);

    await screen.findByText(/title/i);

    expect(screen.queryByText(/contact sales/i)).not.toBeInTheDocument();
  });

  test('team renders cta', async () => {
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = true;

    renderComponent(ctx);

    const link = await screen.findByText(/contact sales/i);
    expect(link.parentElement).toHaveAttribute(
      'href',
      expect.stringMatching(/upgrade-team/i)
    );
  });

  test('eub WITHOUT igs renders cta', async () => {
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = false;

    renderComponent(ctx);

    const link = await screen.findByText(/contact sales/i);
    expect(link.parentElement).toHaveAttribute(
      'href',
      expect.stringMatching(/upgrade-igs/i)
    );
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <CreateAccessList />
      </ContextProvider>
    </MemoryRouter>
  );
}

test('getEligibleUsers: empty', async () => {
  expect(getEligibleUsers([], {}, [])).toStrictEqual([]);

  expect(
    getEligibleUsers([], {}, [
      { label: 'foo', value: { roles: ['access'] } as any },
      { label: 'bar', value: { roles: ['editor'] } },
      { label: 'baz', value: { roles: ['access'] } },
    ])
  ).toStrictEqual([]);
});

test('getEligibleUsers: match by roles only (empty traits)', async () => {
  expect(
    getEligibleUsers([{ label: 'access', value: 'access' }], {}, [
      { label: 'foo', value: { roles: ['access'] } as any },
      { label: 'bar', value: { roles: ['editor'] } },
      { label: 'baz', value: { roles: ['access'] } },
    ])
  ).toStrictEqual([
    { label: 'foo', value: { roles: ['access'] } },
    { label: 'baz', value: { roles: ['access'] } },
  ]);
});

test('getEligibleUsers: match by traits only (empty roles)', async () => {
  expect(
    getEligibleUsers([], { fruit: ['apple'] }, [
      {
        label: 'foo',
        value: { allTraits: { fruit: ['apple'] } } as any,
      },
      { label: 'bar', value: { allTraits: { fruit: ['banana'] } } },
      { label: 'baz', value: { allTraits: { fruit: ['apple'] } } },
    ])
  ).toStrictEqual([
    { label: 'foo', value: { allTraits: { fruit: ['apple'] } } },
    { label: 'baz', value: { allTraits: { fruit: ['apple'] } } },
  ]);
});

test('getEligibleUsers: match by both roles and traits', async () => {
  expect(
    getEligibleUsers(
      [{ label: 'access', value: 'access' }], // rolesRequired
      { fruit: ['apple', 'banana'] }, // traitsRequired
      [
        {
          label: 'foo',
          value: { roles: ['access'], allTraits: {} } as any,
        },
        {
          label: 'bar',
          value: {
            roles: ['access'],
            allTraits: { fruit: ['apple', 'banana'] },
          },
        },
        {
          label: 'baz',
          value: { roles: [], allTraits: { fruit: ['apple'] } },
        },
        {
          label: 'lux',
          value: {
            roles: ['access'],
            allTraits: {},
          },
        },
        {
          label: 'qux',
          value: {
            roles: ['editor', 'access'],
            allTraits: { fruit: ['apple', 'banana'] },
          },
        },
      ]
    )
  ).toStrictEqual([
    {
      label: 'bar',
      value: { roles: ['access'], allTraits: { fruit: ['apple', 'banana'] } },
    },
    {
      label: 'qux',
      value: {
        roles: ['editor', 'access'],
        allTraits: { fruit: ['apple', 'banana'] },
      },
    },
  ]);
});

test('getEligibleUsersAmongSelectedUsers: empty', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({ eligibleUsers: [], selectedUsers: [] })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers: no eligible users', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [],
      selectedUsers: [{ value: { name: 'foo' } } as any],
    })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers: no selected users are eligible', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [{ value: { name: 'bar' } } as any],
      selectedUsers: [{ value: { name: 'foo' } } as any],
    })
  ).toStrictEqual([]);
});

test('getEligibleUsersAmongSelectedUsers', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'baz' } },
      ],
      selectedUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'bar' } },
        { value: { name: 'baz' } },
      ],
    })
  ).toStrictEqual([
    { value: { name: 'foo' } } as any,
    { value: { name: 'baz' } },
  ]);
});

test('getEligibleUsersAmongSelectedUsers ignore options that contain string as a value', async () => {
  expect(
    getEligibleUsersAmongSelectedUsers({
      eligibleUsers: [
        { value: { name: 'foo' } } as any,
        { value: { name: 'baz' } },
      ],
      selectedUsers: [
        { value: { name: 'foo' } } as any,
        { value: 'manual-user-1' },
        { value: { name: 'bar' } },
        { value: { name: 'baz' } },
        { value: 'manual-user-2' },
      ],
    })
  ).toStrictEqual([
    { value: { name: 'foo' } } as any,
    { value: { name: 'baz' } },
  ]);
});
