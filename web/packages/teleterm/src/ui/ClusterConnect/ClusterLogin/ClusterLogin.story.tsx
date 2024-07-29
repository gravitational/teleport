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

import React, { PropsWithChildren } from 'react';

import { Box } from 'design';
import { Attempt, makeErrorAttempt } from 'shared/hooks/useAsync';

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
  title: 'Teleterm/ModalsHost/ClusterLogin',
};

function makeProps(): ClusterLoginPresentationProps {
  return {
    shouldPromptSsoStatus: false,
    title: 'localhost',
    loginAttempt: {
      status: '',
      statusText: '',
    } as Attempt<void>,
    initAttempt: {
      status: 'success',
      statusText: '',
      data: {
        preferredMfa: 'webauthn',
        localAuthEnabled: true,
        authProviders: [],
        type: '',
        secondFactor: 'optional',
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
    webauthnLogin: null,
    reason: undefined,
  };
}

export const Err = () => {
  const props = makeProps();
  props.initAttempt = makeErrorAttempt(new Error('some error message'));

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const Processing = () => {
  const props = makeProps();
  props.initAttempt.status = 'processing';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalDisabled = () => {
  const props = makeProps();
  props.initAttempt.data.localAuthEnabled = false;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalOnly = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
  props.initAttempt.data.allowPasswordless = false;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalOnlyWithReasonGatewayCertExpiredWithDbGateway = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
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

export const LocalOnlyWithReasonGatewayCertExpiredWithKubeGateway = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
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

export const LocalOnlyWithReasonGatewayCertExpiredWithoutGateway = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
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

export const LocalOnlyWithReasonVnetCertExpired = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
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

const authProviders = [
  { type: 'github', name: 'github', displayName: 'GitHub' },
  { type: 'saml', name: 'microsoft', displayName: 'Microsoft' },
];

export const SsoOnly = () => {
  const props = makeProps();
  props.initAttempt.data.localAuthEnabled = false;
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalWithPasswordless = () => {
  return (
    <TestContainer>
      <ClusterLoginPresentation {...makeProps()} />
    </TestContainer>
  );
};

export const LocalLoggedInUserWithPasswordless = () => {
  const props = makeProps();
  props.loggedInUserName = 'llama';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalWithSso = () => {
  const props = makeProps();
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const PasswordlessWithLocal = () => {
  const props = makeProps();
  props.initAttempt.data.localConnectorName = 'passwordless';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const PasswordlessWithLocalLoggedInUser = () => {
  const props = makeProps();
  props.initAttempt.data.localConnectorName = 'passwordless';
  props.loggedInUserName = 'llama';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoWithLocalAndPasswordless = () => {
  const props = makeProps();
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoWithNoProvidersConfigured = () => {
  const props = makeProps();
  props.initAttempt.data.authType = 'github';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwareTapPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.webauthnLogin = {
    prompt: 'tap',
  };
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwarePinPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.webauthnLogin = {
    prompt: 'pin',
  };
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwareRetapPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.webauthnLogin = {
    prompt: 'retap',
  };
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwareCredentialPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.webauthnLogin = {
    prompt: 'credential',
    loginUsernames: [
      'apple',
      'banana',
      'blueberry',
      'carrot',
      'durian',
      'pumpkin',
      'strawberry',
    ],
  };
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwareCredentialPromptProcessing = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.webauthnLogin = {
    prompt: 'credential',
    loginUsernames: [
      'apple',
      'banana',
      'blueberry',
      'carrot',
      'durian',
      'pumpkin',
      'strawberry',
    ],
  };
  props.webauthnLogin.processing = true;
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};
export const SsoPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.shouldPromptSsoStatus = true;
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

const TestContainer: React.FC<PropsWithChildren> = ({ children }) => (
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
