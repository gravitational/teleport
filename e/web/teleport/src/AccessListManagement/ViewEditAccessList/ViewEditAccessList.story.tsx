import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { getAcl } from 'teleport/mocks/contexts';

import { http, HttpResponse } from 'msw';

import { StoryObj } from '@storybook/react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import {
  IneligibleStatus,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { ViewEditAccessList } from './ViewEditAccessList';

export default {
  title: 'TeleportE/AccessLists/View',
  decorators: [
    Story => {
      return <Story />;
    },
  ],
};

// Note the disabled buttons.
// Owners are limited to member edits.
export const ViewingAsOwner: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({
            accessList: mockAccessList,
          });
        }),
      ],
    },
  },
  render() {
    return (
      <Provider customAcl={getAcl({ noAccess: true })}>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

// Note the disabled buttons.
// Members can't edit and view other members.
export const ViewingAsMember: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({
            accessList: {
              ...mockAccessList,
              spec: {
                ...mockAccessList.spec,
                // No owners
                owners: [
                  { name: 'owner1', description: 'some description' },
                  {
                    name: 'george.washington@goteleport.com',
                    ineligible_status: IneligibleStatus.MissingRequirements,
                  },
                ],
              },
            },
          });
        }),
      ],
    },
  },
  render() {
    return (
      <Provider customAcl={getAcl({ noAccess: true })}>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

// Note that admin will have access to all actions.
export const ViewingAsAdmin: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({
            accessList: mockAccessList,
          });
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            { name: 'banana' },
            {
              name: 'carrot',
              roles: ['reviewer', 'auditor'],
              allTraits: { fruit: ['carrot'] },
            },
          ]);
        }),
        http.get(cfg.oss.getListRolesUrl(), () => {
          return HttpResponse.json([
            { name: 'admin' },
            { name: 'auditor' },
            { name: 'reviewer' },
            { name: 'access' },
            { name: 'editor' },
          ]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <ViewEditAccessList />
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
            { status: 500 }
          );
        }),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <ViewEditAccessList />
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

const mockAccessList = {
  metadata: {
    name: 'mock-access-list-id',
    labels: {
      'okta/org': 'https://some-url',
    },
  },
  members: [
    {
      name: 'member1',
      joined: '2023-05-24T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      reason: 'some reason',
      added_by: 'lisa@goteleport.com',
      ineligible_status: IneligibleStatus.Expired,
    },
    {
      name: 'member2',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      added_by: 'llama',
    },
    {
      name: 'member3',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '2024-12-12T17:48:15.78579Z',
      added_by: 'llama',
    },
  ],
  spec: {
    title: 'Mock Access List Title',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
    owners: [
      { name: 'owner1', description: 'some description' },
      {
        name: 'george.washington@goteleport.com',
        ineligible_status: IneligibleStatus.MissingRequirements,
      },
      { name: 'llama' }, // owner
    ],

    grants: {
      roles: ['access', 'editor'],
      traits: { fruit: ['apple'] },
    },
    owner_grants: {
      roles: ['admin', 'almighty'],
      traits: { status: ['pro'] },
    },
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneMonth,
        dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
      },
      next_audit_date: new Date().toString(),
    },
    ownership_requires: {
      roles: ['admin'],
      traits: { fruit: ['banana', 'apple'], drink: ['coffee'] },
    },
    membership_requires: {
      roles: ['reviewer', 'auditor'],
      traits: { fruit: ['carrot'] },
    },
  },
};
