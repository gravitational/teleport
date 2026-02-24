import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { generatePath } from 'react-router';

import { Alert } from 'design/Alert';

import cfg from 'e-teleport/config';
import { getAcl } from 'teleport/mocks/contexts';

import {
  rawAccessList,
  rawAccessListAsMember,
  rawAccessListAsOwner,
  rawAccessListEntraID,
  rawAccessListOkta,
  rawAccessListScim,
  rawAccessListStatic,
  rawEmptyAccessList,
  rawNestedAccessList,
  rawReviewsResponse,
} from './fixtures';
import { Provider } from './TestHelper/Provider';
import { ViewEditAccessList } from './ViewEditAccessList';

export default {
  title: 'TeleportE/AccessLists/View/Default',
};

export const ViewingAsOwnerWithNoRbac: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessManagementListUrl(rawAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListAsOwner,
            });
          }
        ),
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessListAsOwner.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListAsOwner, rawNestedAccessList],
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
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: largely read-only, but can edit members, start reviews, and view
          past audits
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsMember: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListAsMember,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListAsMember, rawNestedAccessList],
            })
          );
        }),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListAsMember,
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
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: read-only, no member list, no audit review list, no starting
          reviews, but can see owners list
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdmin: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessList.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessList,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessList, rawNestedAccessList],
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
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
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
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">Devs: can perform any action</Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdminOktaList: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessListOkta.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessListOkta.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListOkta,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListOkta, rawNestedAccessList],
            })
          );
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            {
              name: 'carrot',
            },
          ]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: has okta badge next to title, can&apos;t modify title and
          can&apos;t modify granted permissions
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdminStaticList: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessListStatic.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessListStatic.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListStatic,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListStatic, rawNestedAccessList],
            })
          );
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            {
              name: 'carrot',
            },
          ]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: rawAccessListStatic.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: no audit tab, all buttons are disabled with appropriate tips
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdminScimList: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessListScim.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessListScim.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListScim,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListScim, rawNestedAccessList],
            })
          );
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            {
              name: 'carrot',
            },
          ]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: rawAccessListScim.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: member list add and delete buttons are disabled
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdminEntraIDList: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawAccessListEntraID.metadata.name },
          }),
          () => {
            return HttpResponse.json(rawReviewsResponse);
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawAccessListEntraID.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawAccessListEntraID,
            });
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawNestedAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawNestedAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [rawAccessListEntraID, rawNestedAccessList],
            })
          );
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            {
              name: 'carrot',
            },
          ]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: rawAccessListEntraID.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: member list add and delete buttons are disabled, Entra ID badge
          next to title
        </Alert>
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const ViewingAsAdminEmptyList: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(
          cfg.getAccessListUrl({
            action: 'reviews',
            params: { accessListId: rawEmptyAccessList.metadata.name },
          }),
          () => {
            return HttpResponse.json({});
          }
        ),
        http.get(
          cfg.getAccessManagementListUrl(rawEmptyAccessList.metadata.name),
          () => {
            return HttpResponse.json({
              accessList: rawEmptyAccessList,
            });
          }
        ),
        http.get(cfg.getAccessManagementListUrl(), () => {
          return new HttpResponse(
            JSON.stringify({
              accessLists: [],
            })
          );
        }),
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <Alert kind="neutral">
          Devs: only title and audit review frequency and date is defined, but
          can perform any action
        </Alert>
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
          cfg.getAccessManagementListUrl(rawAccessList.metadata.name),
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
            accessListId: rawAccessList.metadata.name,
          }),
        ]}
      >
        <ViewEditAccessList />
      </Provider>
    );
  },
};
