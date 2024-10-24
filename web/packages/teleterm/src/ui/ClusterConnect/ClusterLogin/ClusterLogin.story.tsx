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

import { FC, PropsWithChildren } from 'react';

import { Box } from 'design';
import {
  Attempt,
  makeErrorAttempt,
  makeProcessingAttempt,
} from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/clusters/types';

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

export const LocalOnly = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const InitErr = () => {
  const props = makeProps();
  props.initAttempt = makeErrorAttempt(new Error('some error message'));

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const InitProcessing = () => {
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

// The password field is empty in this story as there's no way to change the value of a controlled
// input without touching the internals of React.
// https://stackoverflow.com/questions/23892547/what-is-the-best-way-to-trigger-change-or-input-event-in-react-js
export const LocalProcessing = () => {
  const props = makeProps();
  props.initAttempt.data.allowPasswordless = false;
  props.loginAttempt = makeProcessingAttempt();
  props.loggedInUserName = 'alice';

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
  props.passwordlessLoginState = {
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
  props.passwordlessLoginState = {
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
  props.passwordlessLoginState = {
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
  props.passwordlessLoginState = {
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
  props.passwordlessLoginState = {
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
  props.passwordlessLoginState.processing = true;
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
