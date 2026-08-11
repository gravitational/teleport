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

import cfg from 'teleport/config';
import {
  RequiredDiscoverProviders,
  resourceSpecConnectMyComputer,
} from 'teleport/Discover/Fixtures/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/ConnectMyComputer/TestConnection',

  beforeEach({ msw }) {
    msw.use(
      http.post(cfg.api.webRenewTokenPath, () => HttpResponse.json({})),
      http.post(cfg.getMfaRequiredUrl(cfg.proxyCluster), () =>
        HttpResponse.json({ required: false })
      ),
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
      )
    );
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

SingleLogin.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.connectMyComputerLoginsPath, () =>
      HttpResponse.json({ logins: ['foo'] })
    )
  );
};

export const MultipleLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

MultipleLogins.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.connectMyComputerLoginsPath, () =>
      HttpResponse.json({
        logins: [
          'foo',
          'bar',
          'baz',
          'czesława_maria_de_domo_cieślak_primo_voto_gospodarek_secundo_voto_kowalczyk',
        ],
      })
    )
  );
};

export const NoLogins = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoLogins.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.connectMyComputerLoginsPath, () =>
      HttpResponse.json({ logins: [] })
    )
  );
};

export const NoRole = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

NoRole.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.connectMyComputerLoginsPath, () =>
      HttpResponse.json(
        {
          error: { message: 'No role found' },
        },
        { status: 404 }
      )
    )
  );
};

export const ReloadUserProcessing = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserProcessing.beforeEach = ({ msw }) => {
  msw.use(
    http.post(cfg.api.webRenewTokenPath, async () => await delay('infinite'))
  );
};

export const ReloadUserError = () => {
  return (
    <Provider>
      <TestConnection {...agentStepProps} />
    </Provider>
  );
};

ReloadUserError.beforeEach = ({ msw }) => {
  msw.use(
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
    })
  );
};

const Provider = ({ children }) => {
  return (
    <RequiredDiscoverProviders
      agentMeta={agentStepProps.agentMeta}
      resourceSpec={resourceSpecConnectMyComputer}
    >
      {children}
    </RequiredDiscoverProviders>
  );
};
