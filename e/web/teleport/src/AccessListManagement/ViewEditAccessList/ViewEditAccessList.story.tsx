import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { generatePath, MemoryRouter } from 'react-router';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessListMemberKind,
  IneligibleStatus,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';
import { Route, Switch } from 'teleport/components/Router';
import { getAcl } from 'teleport/mocks/contexts';

import { ViewEditAccessList } from './ViewEditAccessList';

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
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'mock-nested-access-list-id',
      joined: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
      expires: '',
      added_by: 'maxim@goteleport.com',
      membership_kind: AccessListMemberKind.List,
    },
    {
      name: 'member2',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      added_by: 'llama',
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'member3',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: '2024-12-12T17:48:15.78579Z',
      added_by: 'llama',
      membership_kind: AccessListMemberKind.User,
    },
  ],
  // this is a bit of a hack, since we're using the same obj for the list resp and the single resp.
  inherited_member_grants: {
    roles: [],
    traits: {},
  },
  spec: {
    title: 'Mock Access List Title',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
    owners: [
      {
        name: 'owner1',
        description: 'some description',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'george.washington@goteleport.com',
        ineligible_status: IneligibleStatus.MissingRequirements,
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'llama',
        membership_kind: AccessListMemberKind.User,
      }, // owner
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

const mockNestedAccessList = {
  metadata: {
    name: 'mock-nested-access-list-id',
    labels: {},
  },
  members: [
    {
      name: 'member1',
      joined: '2023-05-24T17:48:15.78579Z',
      expires: '0001-01-01T00:00:00Z',
      reason: 'some reason',
      added_by: 'maxim@goteleport.com',
      ineligible_status: IneligibleStatus.Expired,
      membership_kind: AccessListMemberKind.User,
    },
    {
      name: 'member2',
      joined: '2023-12-12T17:48:15.78579Z',
      expires: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
      added_by: 'maxim@goteleport.com',
      ineligible_status: undefined,
      membership_kind: AccessListMemberKind.User,
    },
  ],
  inherited_member_grants: mockAccessList.spec.grants,
  spec: {
    title: 'Mock Nested Access List',
    description:
      'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua',
    owners: [
      {
        name: 'owner1',
        description: 'some description',
        membership_kind: AccessListMemberKind.User,
      },
      {
        name: 'llama',
        membership_kind: AccessListMemberKind.User,
      },
    ],
    grants: {
      roles: ['access'],
      traits: { fruit: ['orange'] },
    },
    owner_grants: {
      roles: [],
      traits: {},
    },
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
      },
      next_audit_date: new Date(
        Date.now() + 365 * 24 * 60 * 60 * 1000
      ).toISOString(),
    },
    ownership_requires: {
      roles: [],
      traits: {},
    },
    membership_requires: {
      roles: [],
      traits: {},
    },
  },
};

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
        http.get(
          cfg.getAccessManagementListUrl(mockAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: mockAccessList,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(mockNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: mockNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [mockAccessList, mockNestedAccessList],
            })
          );
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        customAcl={getAcl({ noAccess: true })}
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: mockAccessList.metadata.name,
          }),
        ]}
      >
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
        http.get(
          cfg.getAccessManagementListUrl(mockNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: mockNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [
                {
                  ...mockAccessList,
                  spec: {
                    ...mockAccessList.spec,
                    owners: [
                      {
                        name: 'owner1',
                        description: 'some description',
                        membership_kind: AccessListMemberKind.User,
                      },
                      {
                        name: 'george.washington@goteleport.com',
                        ineligible_status: IneligibleStatus.MissingRequirements,
                        membership_kind: AccessListMemberKind.User,
                      },
                    ],
                  },
                },
                mockNestedAccessList,
              ],
            })
          );
        }),
        http.get(
          cfg.getAccessManagementListUrl(mockAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: {
                ...mockAccessList,
                spec: {
                  ...mockAccessList.spec,
                  // No owners
                  owners: [
                    {
                      name: 'owner1',
                      description: 'some description',
                      membership_kind: AccessListMemberKind.User,
                    },
                    {
                      name: 'george.washington@goteleport.com',
                      ineligible_status: IneligibleStatus.MissingRequirements,
                      membership_kind: AccessListMemberKind.User,
                    },
                  ],
                },
              },
            });
          }
        ),
      ],
    },
  },
  render() {
    return (
      <Provider
        customAcl={getAcl({ noAccess: true })}
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: mockAccessList.metadata.name,
          }),
        ]}
      >
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
        http.get(
          cfg.getAccessManagementListUrl(mockAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: mockAccessList,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(mockNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: mockNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [mockAccessList, mockNestedAccessList],
            })
          );
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
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: mockAccessList.metadata.name,
          }),
        ]}
      >
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessManagementListUrl(mockAccessList.metadata.name),
          () => {
            return HttpResponse.json(
              {
                error: { message: 'Whoops, something went wrong.' },
              },
              { status: 500 }
            );
          }
        ),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: mockAccessList.metadata.name,
          }),
        ]}
      >
        <ViewEditAccessList />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter initialEntries={props.initialEntries ?? []}>
      <ContextProvider ctx={ctx}>
        <Switch>
          <AccessListManagementContextProvider>
            <Route
              path={cfg.routes.accessLists}
              render={() => <>{props.children}</>}
            />
          </AccessListManagementContextProvider>
        </Switch>
      </ContextProvider>
    </MemoryRouter>
  );
};
