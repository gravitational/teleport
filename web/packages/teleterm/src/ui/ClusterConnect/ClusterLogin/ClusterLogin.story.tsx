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

import { Attempt } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/clusters/types';

import { ClusterLoginPresentation } from './ClusterLogin';

export default {
  title: 'Teleterm/ClusterLogin',
};

function makeProps() {
  return {
    shouldPromptSsoStatus: false,
    shouldPromptHardwareKey: false,
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
      } as types.AuthSettings,
    } as const,

    loggedInUserName: null,
    onCloseDialog: () => null,
    onAbort: () => null,
    onLoginWithLocal: () => Promise.resolve<[void, Error]>([null, null]),
    onLoginWithSso: () => null,
  };
}

export const Basic = () => {
  return <ClusterLoginPresentation {...makeProps()} />;
};

export const HardwareKeyPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.shouldPromptHardwareKey = true;
  return <ClusterLoginPresentation {...props} />;
};

export const SsoPrompt = () => {
  const props = makeProps();
  props.loginAttempt.status = 'processing';
  props.shouldPromptSsoStatus = true;
  return <ClusterLoginPresentation {...props} />;
};
