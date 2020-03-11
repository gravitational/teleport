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
import cfg from 'teleport/config';
import { Session } from 'teleport/services/ssh';
import { TermEventEnum } from 'teleport/lib/term/enums';
import Tty from 'teleport/lib/term/tty';
import ConsoleContext from 'teleport/console/consoleContext';
import { useConsoleContext } from 'teleport/console/consoleContextProvider';

import { DocumentSsh } from 'teleport/console/stores';

export default function useSshSession(doc: DocumentSsh) {
  const { clusterId, sid, serverId, login } = doc;
  const ctx = useConsoleContext();
  const ttyRef = React.useRef<Tty>(null);
  const [session, setSession] = React.useState<Session>(null);
  const [statusText, setStatusText] = React.useState('');
  const [status, setStatus] = React.useState<Status>('loading');

  React.useEffect(() => {
    // initializes tty instances
    function initTty(session: Session) {
      const tty = ctx.createTty(session);

      // susbscribe to tty events to handle connect/disconnects events
      tty.on(TermEventEnum.CLOSE, () => handleTtyDisconnect(ctx, doc.id));
      tty.on(TermEventEnum.CONN_CLOSE, () => handleTtyDisconnect(ctx, doc.id));
      tty.on('open', () => handleTtyConnect(ctx, session, doc.id));

      // assign tty reference so it can be passed down to xterm
      ttyRef.current = tty;
      setSession(session);
      setStatus('initialized');
    }

    // cleanup by unsubscribing from tty
    function cleanup() {
      ttyRef.current && ttyRef.current.removeAllListeners();
    }

    if (sid) {
      // join existing session
      ctx
        .fetchSshSession(clusterId, sid)
        .then(initTty)
        .catch(err => {
          setStatus('notfound');
          setStatusText(err.message);
        });
    } else {
      // create new ssh session
      ctx
        .createSshSession(clusterId, serverId, login)
        .then(initTty)
        .catch(err => {
          setStatus('error');
          setStatusText(err.message);
        });
    }

    return cleanup;
  }, []);

  return {
    tty: ttyRef.current as ReturnType<typeof ctx.createTty>,
    status,
    statusText,
    session,
  };
}

function handleTtyConnect(
  ctx: ConsoleContext,
  session: Session,
  docId: number
) {
  const { hostname, login, sid, clusterId } = session;
  const url = cfg.getConsoleSessionRoute({ sid, clusterId });
  ctx.updateSshDocument(docId, {
    title: `${login}@${hostname}`,
    status: 'connected',
    url,
    ...session,
  });

  ctx.navigateTo({ url }, true);
}

function handleTtyDisconnect(ctx: ConsoleContext, docId: number) {
  ctx.updateSshDocument(docId, {
    status: 'disconnected',
  });
}

type Status = 'initialized' | 'loading' | 'notfound' | 'error';
