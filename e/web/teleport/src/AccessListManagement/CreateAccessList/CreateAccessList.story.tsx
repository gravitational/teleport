import { StoryObj } from '@storybook/react';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';

import { CreateAccessList } from './CreateAccessList';

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
        http.get(cfg.oss.getListRolesUrl(), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json(
            {
              error: { message: 'Whoops, something went wrong.' },
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
        http.get(cfg.oss.api.listRolesPath, () => {
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
        http.get(cfg.oss.getListRolesUrl(), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({ accessLists: [] });
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
        http.get(cfg.oss.getListRolesUrl(), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.getAccessManagementListUrl(), () => {
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

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          {props.children}
        </AccessListManagementContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
