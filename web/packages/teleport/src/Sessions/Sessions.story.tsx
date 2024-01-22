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

import React from 'react';
import { MemoryRouter } from 'react-router';

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
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function ActiveSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showActiveSessionsCTA: true,
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function ModeratedSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showModeratedSessionsCTA: true,
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

const ctx = createTeleportContext();

const makeSessionProps = (
  overrides: Partial<typeof useSessions> = {}
): ReturnType<typeof useSessions> => {
  return Object.assign(
    {
      ctx,
      clusterId: 'teleport.example.sh',
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
