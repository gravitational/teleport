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
import { Meta } from '@storybook/react';

import { makeErrorAttempt, makeProcessingAttempt } from 'shared/hooks/useAsync';

import { ClusterLoginPresentation } from './ClusterLogin';
import {
  compatibilityArgType,
  makeProps,
  StoryProps,
  TestContainer,
} from './storyHelpers';

const meta: Meta<StoryProps> = {
  title: 'Teleterm/ModalsHost/ClusterLogin',
  argTypes: compatibilityArgType,
  args: {
    compatibility: 'compatible',
  },
};
export default meta;

export const LocalOnly = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const InitErr = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt = makeErrorAttempt(new Error('some error message'));

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const InitProcessing = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.status = 'processing';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalDisabled = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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
export const LocalProcessing = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.loginAttempt = makeProcessingAttempt();
  props.loggedInUserName = 'alice';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalError = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.loginAttempt = makeErrorAttempt(new Error('invalid credentials'));
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

export const SsoOnly = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.localAuthEnabled = false;
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoPrompt = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.loginAttempt.status = 'processing';
  props.shouldPromptSsoStatus = true;
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoError = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.localAuthEnabled = false;
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProviders = authProviders;
  props.loginAttempt = makeErrorAttempt(
    new Error("Failed to log in. Please check Teleport's log for more details.")
  );
  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalWithPasswordless = (storyProps: StoryProps) => {
  return (
    <TestContainer>
      <ClusterLoginPresentation {...makeProps(storyProps)} />
    </TestContainer>
  );
};

export const LocalLoggedInUserWithPasswordless = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.loggedInUserName = 'llama';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const LocalWithSso = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const PasswordlessWithLocal = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.localConnectorName = 'passwordless';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const PasswordlessWithLocalLoggedInUser = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.localConnectorName = 'passwordless';
  props.loggedInUserName = 'llama';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoWithLocalAndPasswordless = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.authType = 'github';
  props.initAttempt.data.authProviders = authProviders;

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const SsoWithNoProvidersConfigured = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.authType = 'github';

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const HardwareTapPrompt = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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

export const HardwarePinPrompt = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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

export const HardwareRetapPrompt = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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

export const HardwareCredentialPrompt = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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

export const HardwareCredentialPromptProcessing = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
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
