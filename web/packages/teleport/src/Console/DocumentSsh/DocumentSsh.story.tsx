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

import DocumentSsh from './DocumentSsh';
import { TestLayout } from './../Console.story';
import ConsoleCtx from './../consoleContext';

import type { Session } from 'teleport/services/session';

export const Connected = () => {
  const ctx = new ConsoleCtx();
  const tty = ctx.createTty(session);
  tty.connect = () => null;
  ctx.createTty = () => tty;

  return (
    <TestLayout ctx={ctx}>
      <DocumentSsh doc={doc} visible={true} />
    </TestLayout>
  );
};

export const NotFound = () => {
  const ctx = new ConsoleCtx();
  const tty = ctx.createTty(session);
  tty.connect = () => null;
  ctx.createTty = () => tty;

  const disconnectedDoc = {
    ...doc,
    status: 'disconnected' as const,
  };

  return (
    <TestLayout ctx={ctx}>
      <DocumentSsh doc={disconnectedDoc} visible={true} />
    </TestLayout>
  );
};

export const ServerError = () => {
  const ctx = new ConsoleCtx();
  const tty = ctx.createTty(session);
  tty.connect = () => null;
  ctx.createTty = () => tty;
  const noSidDoc = {
    ...doc,
    sid: '',
  };

  return (
    <TestLayout ctx={ctx}>
      <DocumentSsh doc={noSidDoc} visible={true} />
    </TestLayout>
  );
};

export default {
  title: 'Teleport/Console/DocumentSsh',
};

const doc = {
  kind: 'terminal',
  status: 'connected',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  login: 'login-value',
  id: 3,
  url: 'fd',
  created: new Date(),
  latency: {
    client: 123,
    server: 456,
  },
} as const;

const session: Session = {
  kind: 'ssh',
  login: '123',
  sid: '',
  namespace: '',
  created: new Date(),
  durationText: '',
  serverId: '',
  resourceName: '',
  clusterId: '',
  parties: [],
  addr: '1.1.1.1:1111',
  participantModes: ['observer', 'moderator', 'peer'],
  moderated: false,
  command:
    'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
};
