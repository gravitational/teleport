/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { FC, PropsWithChildren } from 'react';
import { Box } from 'design';
import { Attempt } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/clusters/types';
import {
  appUri,
  makeDatabaseGateway,
  makeKubeGateway,
} from 'teleterm/services/tshd/testHelpers';

import {
  ClusterLoginPresentation,
  ClusterLoginPresentationProps,
} from './ClusterLogin';

export default {
  title: 'Teleterm/ModalsHost/ClusterLogin/Reason',
};

export const GatewayCertExpiredWithDbGateway = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: dbGateway.targetUri,
    gateway: dbGateway,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const GatewayCertExpiredWithKubeGateway = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: kubeGateway.targetUri,
    gateway: kubeGateway,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const GatewayCertExpiredWithoutGateway = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: dbGateway.targetUri,
    gateway: undefined,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const VnetCertExpired = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.vnet-cert-expired',
    targetUri: appUri,
    publicAddr: 'tcp-app.teleport.example.com',
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

function makeProps(): ClusterLoginPresentationProps {
  return {
    shouldPromptSsoStatus: false,
    title: 'localhost',
    loginAttempt: {
      status: '',
      statusText: '',
    } as Attempt<void>,
    init: () => null,
    initAttempt: {
      status: 'success',
      statusText: '',
      data: {
        localAuthEnabled: true,
        authProviders: [],
        type: '',
        hasMessageOfTheDay: false,
        allowPasswordless: true,
        localConnectorName: '',
        authType: 'local',
      } as types.AuthSettings,
    } as const,

    loggedInUserName: null,
    onCloseDialog: () => null,
    onAbort: () => null,
    onLoginWithLocal: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithPasswordless: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithSso: () => null,
    clearLoginAttempt: () => null,
    passwordlessLoginState: null,
    reason: undefined,
  };
}

const TestContainer: FC<PropsWithChildren> = ({ children }) => (
  <>
    <span>Bordered box is not part of the component</span>
    <Box
      css={`
        width: 450px;
        border: 1px solid ${props => props.theme.colors.levels.elevated};
        background: ${props => props.theme.colors.levels.surface};
      `}
    >
      {children}
    </Box>
  </>
);

const dbGateway = makeDatabaseGateway({
  uri: '/gateways/gateway1',
  targetName: 'postgres',
  targetUri: '/clusters/teleport-local/dbs/postgres',
  targetUser: 'alice',
  targetSubresourceName: '',
  localAddress: 'localhost',
  localPort: '59116',
  protocol: 'postgres',
});

const kubeGateway = makeKubeGateway({
  uri: '/gateways/gateway2',
  targetName: 'minikube',
  targetUri: '/clusters/teleport-local/kubes/minikube',
  targetSubresourceName: '',
  localAddress: 'localhost',
  localPort: '59117',
});
