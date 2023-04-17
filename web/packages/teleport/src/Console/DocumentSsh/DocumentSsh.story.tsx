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
};
