import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { fullAccess } from 'teleport/mocks/contexts';
import { makeAcl } from 'teleport/services/user/makeAcl';

import { CreateAccessListContextProvider } from '../CreateAccessListContextProvider';
import { SelectGuide } from './SelectGuide';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;

export default {
  title: 'TeleportE/AccessLists/Guide/Select',
  decorators: [
    Story => {
      cfg.oss.isEnterprise = true;
      cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

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

export const NoAccessListAccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
      ],
    },
  },
  render() {
    const acl = makeAcl({});
    acl.roles = fullAccess;
    return (
      <Provider customAcl={acl}>
        <SelectGuide />
      </Provider>
    );
  },
};

export const NoRoleAccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
      ],
    },
  },
  render() {
    const acl = makeAcl({});
    acl.accessList = fullAccess;
    return (
      <Provider customAcl={acl}>
        <SelectGuide />
      </Provider>
    );
  },
};

export const NoRoleAccessPartial: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
      ],
    },
  },
  render() {
    const acl = makeAcl({});
    acl.accessList = fullAccess;
    acl.roles = { ...fullAccess, create: false, remove: false };
    return (
      <Provider customAcl={acl}>
        <SelectGuide />
      </Provider>
    );
  },
};

export const FullAccessWithNoLicenseLimits: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    return (
      <Provider>
        <SelectGuide />
      </Provider>
    );
  },
};

export const FullAccessWithLicenseLimitReached: StoryObj = {
  parameters: {
    msw: {
      handlers: [
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
        <SelectGuide />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <CreateAccessListContextProvider>
              {props.children}
            </CreateAccessListContextProvider>
          </AccessListManagementContextProvider>
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
};
