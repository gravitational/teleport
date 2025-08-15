/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory } from 'history';
import { MemoryRouter, Route, Router } from 'react-router';

import Box from 'design/Box';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { listBotInstancesSuccess } from 'teleport/test/helpers/botInstances';
import {
  deleteBotSuccess,
  editBotSuccess,
  getBotError,
  getBotForever,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import {
  createLockSuccess,
  listV2LocksError,
  listV2LocksSuccess,
  removeLockSuccess,
} from 'teleport/test/helpers/locks';
import { mfaAuthnChallengeSuccess } from 'teleport/test/helpers/mfa';
import { successGetRoles } from 'teleport/test/helpers/roles';
import {
  listV2TokensError,
  listV2TokensMfaError,
  listV2TokensSuccess,
} from 'teleport/test/helpers/tokens';

import { BotDetails } from './BotDetails';
import { listBotInstancesSuccessHandler } from './InstancesPanel.story';

const meta = {
  title: 'Teleport/Bots/Details',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

const successHandler = getBotSuccess({
  name: 'ansible-worker',
  roles: Array.from({ length: 8 }, (_, k) => k).map(
    r => `testing-role-${r + 1}`
  ),
  traits: [
    {
      name: 'logins',
      values: ['guest'],
    },
    {
      name: 'db_names',
      values: ['*'],
    },
    {
      name: 'custom_idp',
      values: Array.from({ length: 8 }, (_, k) => k).map(
        r => `test-value-${r + 1}`
      ),
    },
  ],
  max_session_ttl: {
    seconds: 43200,
  },
});

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: Array.from({ length: 10 }, (_, k) => k).map(r => ({
            content: `role-${r}`,
            id: `role-${r}`,
            name: `role-${r}`,
            kind: 'role',
          })),
        }),
        listV2LocksSuccess(),
        editBotSuccess(),
        removeLockSuccess(),
        createLockSuccess(),
        deleteBotSuccess(),
      ],
    },
  },
};

export const HappyWithEmpty: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotSuccess({
          name: 'ansible-worker',
          roles: [],
          traits: [],
          max_session_ttl: {
            seconds: 43200,
          },
        }),
        listV2TokensSuccess({
          tokens: [],
        }),
        mfaAuthnChallengeSuccess(),
        listBotInstancesSuccess({
          bot_instances: [],
          next_page_token: '',
        }),
        successGetRoles({
          startKey: '',
          items: Array.from({ length: 10 }, (_, k) => k).map(r => ({
            content: `role-${r}`,
            id: `role-${r}`,
            name: `role-${r}`,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithTypical: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotSuccess({
          name: 'ansible-worker',
          roles: ['access'],
          traits: [],
          max_session_ttl: {
            seconds: 43200,
          },
        }),
        listV2TokensSuccess({
          tokens: ['kubernetes'],
        }),
        mfaAuthnChallengeSuccess(),
        listBotInstancesSuccess({
          bot_instances: [
            {
              bot_name: 'bot-1',
              instance_id: '6570dbf1-3530-4e13-a8c7-497bb9927994',
              active_at_latest: new Date().toISOString(),
              host_name_latest:
                'my-svc.my-namespace.svc.cluster-domain.example',
              join_method_latest: 'kubernetes',
              os_latest: 'linux',
              version_latest: '18.1.0',
            },
          ],
          next_page_token: '',
        }),
        successGetRoles({
          startKey: '',
          items: Array.from({ length: 10 }, (_, k) => k).map(r => ({
            content: `role-${r}`,
            id: `role-${r}`,
            name: `role-${r}`,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithLongValues: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotSuccess({
          name: 'ansibleworkeransibleworkeransibleworkeransibleworkeransibleworkeransibleworker',
          roles: [
            'rolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerolerole',
          ],
          traits: [
            {
              name: 'traittraittraittraittraittraittraittraittraittraittraittraittraittraittrait',
              values: ['value'],
            },
            {
              name: 'name',
              values: [
                'valuevaluevaluevaluevaluevaluevaluevaluevaluevaluevaluevaluevaluevaluevaluevalue',
              ],
            },
          ],
          max_session_ttl: {
            seconds: 43200,
          },
        }),
        listV2TokensSuccess({
          tokens: [
            'tokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentokentoken',
          ],
        }),
        mfaAuthnChallengeSuccess(),
        listBotInstancesSuccess({
          bot_instances: [
            {
              bot_name: '',
              instance_id:
                '04241a2a66b904241a2a66b904241a2a66b904241a2a66b904241a2a66b9',
              host_name_latest:
                'hotnamehotnamehotnamehotnamehotnamehotnamehotnamehotnamehotname',
              active_at_latest: '2025-01-01T00:00:00Z',
              join_method_latest: 'github',
              os_latest: 'linux',
              version_latest: '17.2.6-04241a2',
            },
          ],
          next_page_token: '',
        }),
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithoutEditPermission: Story = {
  args: {
    hasBotsEdit: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithoutTokenListPermission: Story = {
  args: {
    hasTokensList: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithMFAPrompt: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensMfaError(),
        mfaAuthnChallengeSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithTokensError: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensError(500, 'something went wrong'),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithTokensOutdatedProxy: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensError(404, 'path not found', {
          proxyVersion: {
            major: 19,
            minor: 0,
            patch: 0,
            preRelease: 'dev',
            string: '18.0.0',
          },
        }),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithoutBotInstanceListPermission: Story = {
  args: {
    hasBotInstanceListPermission: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const HappyWithLock: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: Array.from({ length: 10 }, (_, k) => k).map(r => ({
            content: `role-${r}`,
            id: `role-${r}`,
            name: `role-${r}`,
            kind: 'role',
          })),
        }),
        listV2LocksSuccess({
          locks: [
            {
              name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
              message: 'This is a test message',
              expires: '2023-12-31T23:59:59Z',
              targets: {
                user: 'bot-ansible-worker',
              },
              createdAt: '2023-01-01T00:00:00Z',
              createdBy: 'admin',
            },
          ],
        }),
        editBotSuccess(),
        removeLockSuccess(),
        createLockSuccess(),
      ],
    },
  },
};

export const HappyWithLockError: Story = {
  args: {
    hasLocksMutatePermission: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listV2TokensSuccess(),
        listBotInstancesSuccessHandler,
        successGetRoles({
          startKey: '',
          items: Array.from({ length: 10 }, (_, k) => k).map(r => ({
            content: `role-${r}`,
            id: `role-${r}`,
            name: `role-${r}`,
            kind: 'role',
          })),
        }),
        listV2LocksError(500, 'error message goes here'),
        editBotSuccess(),
      ],
    },
  },
};

export const WithFetchPending: Story = {
  parameters: {
    msw: {
      handlers: [getBotForever(), listV2LocksSuccess()],
    },
  },
};

export const WithFetchFailure: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotError(500, 'error message goes here'),
        listV2LocksSuccess(),
      ],
    },
  },
};

export const WithBotNotFound: Story = {
  parameters: {
    msw: {
      handlers: [getBotError(404, 'not found'), listV2LocksSuccess()],
    },
  },
};

export const WithNoBotReadPermission: Story = {
  args: {
    hasBotsRead: false,
  },
  parameters: {
    msw: {
      handlers: [
        getBotError(500, 'you have permission, congrats 🎉'),
        listV2LocksSuccess(),
      ],
    },
  },
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

function Wrapper(props?: {
  hasBotsRead?: boolean;
  hasBotsEdit?: boolean;
  hasBotsDelete?: boolean;
  hasTokensList?: boolean;
  hasBotInstanceListPermission?: boolean;
  hasLocksListPermission?: boolean;
  hasLocksMutatePermission?: boolean;
  hasLocksDeletePermission?: boolean;
}) {
  const {
    hasBotsRead = true,
    hasBotsEdit = true,
    hasBotsDelete = true,
    hasTokensList = true,
    hasBotInstanceListPermission = true,
    hasLocksListPermission = true,
    hasLocksMutatePermission = true,
    hasLocksDeletePermission = true,
  } = props ?? {};

  const history = createMemoryHistory({
    initialEntries: ['/web/bot/ansible-worker'],
  });

  const customAcl = makeAcl({
    bots: {
      ...defaultAccess,
      read: hasBotsRead,
      edit: hasBotsEdit,
      remove: hasBotsDelete,
    },
    roles: {
      ...defaultAccess,
      list: true,
    },
    tokens: {
      ...defaultAccess,
      list: hasTokensList,
    },
    botInstances: {
      ...defaultAccess,
      list: hasBotInstanceListPermission,
    },
    lock: {
      ...defaultAccess,
      list: hasLocksListPermission,
      create: hasLocksMutatePermission,
      edit: hasLocksMutatePermission,
      remove: hasLocksDeletePermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <TeleportProviderBasic teleportCtx={ctx}>
          <Router history={history}>
            <Route path={cfg.routes.bot}>
              <Box height={820} overflow={'auto'}>
                <BotDetails />
              </Box>
            </Route>
          </Router>
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
