import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';

import { CreateAccessList } from './CreateAccessList';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';
import { Finished as FinishedComp } from './Finished';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;

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
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
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
        }),
      ],
    },
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
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.api.usersPath, () => {
          return new HttpResponse();
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return new HttpResponse();
        }),
      ],
    },
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
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
        http.post(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({});
        }),
      ],
    },
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
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
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
        }),
      ],
    },
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

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <CreateAccessListContextProvider
            mockCreatedAccessList={{
              id: 'random-access-list-id-1234',
              title: 'some access list title that is really long',
              type: AccessListType.Default,
              audit: {
                recurrence: {
                  frequency: ReviewFrequency.OneMonth,
                  dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
                },
                nextDate: new Date(),
              },
              grants: { roles: [], traits: {} },
              inheritedMemberGrants: { roles: [], traits: {} },
              ownerGrants: { roles: [], traits: {} },
              ownershipRequires: {
                roles: [],
                traits: {},
              },
              owners: [],
            }}
          >
            {props.children}
          </CreateAccessListContextProvider>
        </AccessListManagementContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
