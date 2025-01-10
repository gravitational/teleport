/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { nodes } from 'teleport/Nodes/fixtures';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/ConnectMyComputer/TestConnection',
  parameters: {
    msw: {
      // All handlers within the story must be specified as keys in order to use Storybook's
      // parameter inheritance to share handlers between stories.
      //
      // https://github.com/mswjs/msw-storybook-addon/tree/v1.10.0#composing-request-handlers
      // https://storybook.js.org/docs/6.5/writing-stories/parameters#rules-of-parameter-inheritance
      handlers: {
        renewToken: http.post(cfg.api.webRenewTokenPath, () =>
          HttpResponse.json({})
        ),
        mfaRequired: [
          http.post(cfg.getMfaRequiredUrl(), () =>
            HttpResponse.json({ required: false })
          ),
        ],
        connectionDiagnostic: [
          http.post(cfg.getConnectionDiagnosticUrl(), () =>
            HttpResponse.json({
              id: '1234',
              success: true,
              traces: [
                {
                  traceType: 'rbac node',
                  status: 'success',
                  details: 'Everything is a-okay.',
                },
              ],
            })
          ),
        ],
      },
    },
  },
};

const node = { ...nodes[0] };
node.sshLogins = [
  ...node.sshLogins,
  'george_washington_really_long_name_testing',
];
const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const SingleLogin = () => (
  <Provider>
    <TestConnection {...agentStepProps} />
  </Provider>
);

SingleLogin.parameters = {
  msw: {
    handlers: {
      connectMyComputerLogins: [
        http.get(cfg.api.connectMyComputerLoginsPath, () =>
          HttpResponse.json({ logins: ['foo'] })
        ),
      ],
    },
  },
};

export const MultipleLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

MultipleLogins.parameters = {
  msw: {
    handlers: {
      connectMyComputerLogins: [
        http.get(cfg.api.connectMyComputerLoginsPath, () =>
          HttpResponse.json({
            logins: [
              'foo',
              'bar',
              'baz',
              'czesława_maria_de_domo_cieślak_primo_voto_gospodarek_secundo_voto_kowalczyk',
            ],
          })
        ),
      ],
    },
  },
};

export const NoLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoLogins.parameters = {
  msw: {
    handlers: {
      connectMyComputerLogins: [
        http.get(cfg.api.connectMyComputerLoginsPath, () =>
          HttpResponse.json({ logins: [] })
        ),
      ],
    },
  },
};

export const NoRole = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoRole.parameters = {
  msw: {
    handlers: {
      connectMyComputerLogins: [
        http.get(cfg.api.connectMyComputerLoginsPath, () =>
          HttpResponse.json(
            {
              error: { message: 'No role found' },
            },
            { status: 404 }
          )
        ),
      ],
    },
  },
};

export const ReloadUserProcessing = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserProcessing.parameters = {
  msw: {
    handlers: {
      renewToken: [
        http.post(
          cfg.api.webRenewTokenPath,
          async () => await delay('infinite')
        ),
      ],
    },
  },
};

export const ReloadUserError = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserError.parameters = {
  msw: {
    handlers: {
      // The first handler returns an error immediately. Subsequent requests return after a delay so
      // that we can show a spinner after clicking on "Retry".
      renewToken: [
        http.post(
          cfg.api.webRenewTokenPath,
          () =>
            HttpResponse.json(
              {
                message: 'Could not renew session',
              },
              { status: 500 }
            ),
          { once: true }
        ),
        http.post(cfg.api.webRenewTokenPath, async () => {
          await delay(1000);
          return HttpResponse.json(
            {
              message: 'Could not renew session',
            },
            { status: 500 }
          );
        }),
      ],
    },
  },
};

const Provider = ({ children }) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    ...agentStepProps,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: undefined,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
