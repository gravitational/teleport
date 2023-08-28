/*
Copyright 2020 Gravitational, Inc.

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

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';
import useSessions from 'teleport/Sessions/useSessions';

import { Sessions } from './Sessions';
import { sessions } from './fixtures';

export default {
  title: 'Teleport/ActiveSessions',
};

export function Loaded() {
  const props = makeSessionProps({ attempt: { isSuccess: true } });

  return (
    <ContextProvider ctx={ctx}>
      <Sessions {...props} />
    </ContextProvider>
  );
}

export function ActiveSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showActiveSessionsCTA: true,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Sessions {...props} />
    </ContextProvider>
  );
}

export function ModeratedSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showModeratedSessionsCTA: true,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Sessions {...props} />
    </ContextProvider>
  );
}

const ctx = createTeleportContext();

const makeSessionProps = (
  overrides: Partial<typeof useSessions> = {}
): ReturnType<typeof useSessions> => {
  return Object.assign(
    {
      sessions,
      attempt: {
        isSuccess: false,
        isProcessing: false,
        isFailed: false,
        message: '',
      },
      showActiveSessionsCTA: false,
      showModeratedSessionsCTA: false,
    },
    overrides
  );
};
