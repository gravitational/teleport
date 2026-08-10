import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';
import { Acl } from 'teleport/services/user';

import { CreateAccessList } from './CreateAccessList';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { Finished as FinishedComp } from './Finished';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;
const rootScopedRolesPath = cfg.getRootScopedRolesUrl({}).split('?')[0];

const getRootScopedRolesHandler = http.get(rootScopedRolesPath, () => {
  return HttpResponse.json({
    roles: [
      {
        name: 'team-admin',
        scope: '/',
        assignableScopes: ['/dev', '/staging/**'],
      },
      {
        name: 'audit',
        scope: '/',
        assignableScopes: ['/**'],
      },
    ],
    startKey: '',
  });
});

export default {
  title: 'TeleportE/AccessLists/Create',
  decorators: [
    Story => {
      cfg.oss.isEnterprise = true;
      // Reset request handlers added in individual stories.
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isEnterprise = defaultIsEnterprise;
          cfg.oss.entitlements.AccessLists = defaultAccessListEntitlement;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const Failed: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
        return HttpResponse.json([]);
      }),
      http.get(cfg.oss.api.usersPath, () => {
        return HttpResponse.json([]);
      }),
      getRootScopedRolesHandler,
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json(
          {
            error: { message: 'Whoops, listing access lists error' },
          },
          { status: 500 }
        );
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return HttpResponse.json(
          {
            error: { message: 'Whoops, creating list error' },
          },
          { status: 500 }
        );
      })
    );
  },

  render() {
    return (
      <Provider>
        <Info>
          Dev: fill out form and click Create button to see failed state
        </Info>
        <CreateAccessList />
      </Provider>
    );
  },
};

export const NoAccess: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.oss.api.usersPath, () => {
        return new HttpResponse();
      }),
      http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
        return new HttpResponse();
      }),
      getRootScopedRolesHandler
    );
  },

  render() {
    return (
      <Provider customAcl={getAcl({ noAccess: true })}>
        <CreateAccessList />
      </Provider>
    );
  },
};

export const LoadedWithoutLimit: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
        return HttpResponse.json([]);
      }),
      http.get(cfg.oss.api.usersPath, () => {
        return HttpResponse.json([]);
      }),
      getRootScopedRolesHandler,
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({ accessLists: [] });
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return HttpResponse.json({});
      })
    );
  },

  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    return (
      <Provider>
        <CreateAccessList />
      </Provider>
    );
  },
};

export const LoadedReachedLimit: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
        return HttpResponse.json([]);
      }),
      http.get(cfg.oss.api.usersPath, () => {
        return HttpResponse.json([]);
      }),
      getRootScopedRolesHandler,
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({
          accessLists: [
            {
              metadata: { name: 'aaa' },
              spec: {
                title: 'Interns',
                description: 'lorem ipsum description',
                audit: { frequency: '', next_audit_date: new Date() },
                grants: { roles: ['access', 'editor'] },
                ownership_requires: { roles: [] },
                owners: [],
              },
              membersCount: 0,
            },
          ],
        });
      })
    );
  },

  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 1 };

    return (
      <Provider>
        <CreateAccessList />
      </Provider>
    );
  },
};

export function Finished() {
  return (
    <Provider>
      <FinishedComp />
    </Provider>
  );
}

export function FinishedLongTerm() {
  return (
    <Provider
      createdAccessList={{
        ...createdAccessList,
        preset: 'long-term',
        metadata: {
          name: 'random-access-list-id-1234',
          labels: {
            'teleport.internal/access-list-preset-roles':
              'requester-acl-preset-random-access-list-id-1234',
          },
          revision: '',
        },
      }}
    >
      <FinishedComp />
    </Provider>
  );
}

const Provider = (props: {
  createdAccessList?: AccessList;
  customAcl?: Acl;
  children: React.ReactNode;
}) => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <CreateAccessListContextProvider
              mockCreatedAccessList={
                props.createdAccessList ?? createdAccessList
              }
            >
              {props.children}
            </CreateAccessListContextProvider>
          </AccessListManagementContextProvider>
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
};

const createdAccessList: AccessList = {
  id: 'random-access-list-id-1234',
  metadata: {
    name: 'random-access-list-id-1234',
    labels: {},
    revision: '',
  },
  title: 'some access list title that is really long',
  type: AccessListType.Default,
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneMonth,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date(),
  },
  grants: { roles: [], traits: {}, scopedRoles: [] },
  inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
  ownerGrants: { roles: [], traits: {}, scopedRoles: [] },
  ownershipRequires: {
    roles: [],
    traits: {},
  },
  owners: [],
};
