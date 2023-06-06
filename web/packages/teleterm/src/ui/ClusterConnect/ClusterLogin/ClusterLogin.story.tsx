/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import { Box } from 'design';
import { Attempt } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/clusters/types';
import { makeGateway } from 'teleterm/services/tshd/testHelpers';

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
        authProvidersList: [],
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

export const Error = () => {
  const props = makeProps();
  props.initAttempt = {
    status: 'error',
    statusText: 'some error message',
  };

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

export const LocalOnlyWithReasonGatewayCertExpiredWithGateway = () => {
  const props = makeProps();
  props.initAttempt.data.secondFactor = 'off';
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: gateway.targetUri,
    gateway: gateway,
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
    targetUri: gateway.targetUri,
    gateway: undefined,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoOnly = () => {
  const props = makeProps();
  props.initAttempt.data.localAuthEnabled = false;
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProvidersList = [
    { type: 'github', name: 'github', displayName: 'github' },
    { type: 'saml', name: 'microsoft', displayName: 'microsoft' },
  ];

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
  props.initAttempt.data.authProvidersList = [
    { type: 'github', name: 'github', displayName: 'github' },
    { type: 'saml', name: 'microsoft', displayName: 'microsoft' },
  ];

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
  props.initAttempt.data.authProvidersList = [
    { type: 'github', name: 'github', displayName: 'github' },
    { type: 'saml', name: 'microsoft', displayName: 'microsoft' },
  ];

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

const TestContainer: React.FC = ({ children }) => (
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

const gateway = makeGateway({
  uri: '/gateways/gateway1',
  targetName: 'postgres',
  targetUri: '/clusters/teleport-local/dbs/postgres',
  targetUser: 'alice',
  targetSubresourceName: '',
  localAddress: 'localhost',
  localPort: '59116',
  protocol: 'postgres',
});
