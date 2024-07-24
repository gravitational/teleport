import React, { useEffect } from 'react';
import { addWeeks } from 'date-fns';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';
import Box from 'design/Box';

import { StoryObj } from '@storybook/react';

import { http, HttpResponse } from 'msw';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'e-teleport/config';

import { AccessLists } from './AccessLists';

const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultAccessListEntitlement = cfg.oss.entitlements.AccessLists;

export default {
  title: 'TeleportE/AccessLists/List',
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

export const ListUnlimited: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new Response(
            JSON.stringify({
              accessLists: mockAccessLists,
            })
          );
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 0 };

    return (
      <Provider>
        <AccessLists />
      </Provider>
    );
  },
};

export const ListLimitedAccessCta: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new Response(JSON.stringify({ accessLists: mockAccessLists }));
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 45 };

    return (
      <Provider>
        <Box>
          <AccessLists />
        </Box>
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json(
            {
              error: { message: 'Whoops, something went wrong.' },
            },
            { status: 400 }
          );
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <AccessLists />
      </Provider>
    );
  },
};

export const NoAccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(null, { status: 403 });
        }),
      ],
    },
  },
  render() {
    return (
      <Provider customAcl={getAcl({ noAccess: true })}>
        <AccessLists />
      </Provider>
    );
  },
};

export const EmptyUnlimitedAccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [],
            })
          );
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    return (
      <Provider>
        <AccessLists />
      </Provider>
    );
  },
};

export const EmptyLimitedAccessCta: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(JSON.stringify({ accessLists: [] }));
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.AccessLists = { enabled: true, limit: 4 };

    return (
      <Provider>
        <AccessLists />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>{props.children}</ContextProvider>
    </MemoryRouter>
  );
};

const mockAccessLists = [
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
  {
    metadata: {
      name: 'bbb',
      labels: {
        'okta/org': 'https://some-url',
      },
    },
    spec: {
      title:
        'Really long title to test ellipsis lorem ipsum dolores george washington',
      description:
        'test long description to test ellipsis lorem ipsum descriptionlorem ipsum descriptionlorem ipsum description lorem ipsum description',
      audit: {},
      grants: {
        roles: [
          'access',
          'editor',
          'admin',
          'auditor',
          'reviewer',
          'foo',
          'bar',
        ],
      },
      ownership_requires: { roles: [] },
      owners: [],
    },
    members: Array(243).fill({}),
    membersCount: 243,
  },
  {
    metadata: { name: 'ccc' },
    spec: {
      title: 'All Employees',
      description: 'lorem ipsum some kind of generic description',
      audit: { frequency: '', next_audit_date: addWeeks(new Date(), 2) },
      grants: {
        roles: ['access'],
        traits: { drink: ['banana', 'carrot', 'apple'] },
      },
      ownership_requires: { roles: [] },
      owners: [],
    },
    members: Array(15).fill({}),
    membersCount: 15,
  },
  {
    metadata: {
      name: 'ddd',
      labels: {
        'okta/org': 'https://some-url',
      },
    },
    spec: {
      title: 'Design Team',
      description: 'lorem ipsum some kind of generic description',
      audit: {},
      grants: { roles: ['design', 'ux', 'ui', 'llama'] },
      ownership_requires: { roles: [] },
      owners: [],
    },
    members: Array(1).fill({}),
    membersCount: 1,
  },
  {
    metadata: { name: 'eee' },
    spec: {
      title: 'Test empty description',
      audit: {},
      grants: { roles: ['test'] },
      ownership_requires: { roles: [] },
      owners: [],
    },
    members: Array(1).fill({}),
    membersCount: 1,
  },
  {
    metadata: {
      name: 'fff',
      labels: {
        'okta/org': 'https://some-url',
      },
    },
    spec: {
      title: 'Kubernetes Access With a Long Name',
      description:
        'test long description to test ellipsis lorem ipsum descriptionlorem ipsum descriptionlorem ipsum description lorem ipsum description',
      audit: { frequency: '', next_audit_date: addWeeks(new Date(), 1) },
      grants: {
        roles: [
          'reallyreallyobnoxiouslonglabeltesting',
          'reallyreallyobnoxiouslonglabeltesting',
          'reallyreallyobnoxiouslonglabeltesting',
        ],
      },
      ownership_requires: { roles: [] },
      owners: [],
    },
    members: Array(20000).fill({}),
    membersCount: 20000,
  },
];
