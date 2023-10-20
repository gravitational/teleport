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

import { context, trace } from '@opentelemetry/api';

import cfg from 'teleport/config';
import { TermEvent } from 'teleport/lib/term/enums';
import Tty from 'teleport/lib/term/tty';
import ConsoleContext from 'teleport/Console/consoleContext';
import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import { DocumentSsh } from 'teleport/Console/stores';

import type {
  ParticipantMode,
  Session,
  SessionMetadata,
} from 'teleport/services/session';

const tracer = trace.getTracer('TTY');

export default function useSshSession(doc: DocumentSsh) {
  const { clusterId, sid, serverId, login, mode } = doc;
  const ctx = useConsoleContext();
  const ttyRef = React.useRef<Tty>(null);
  const tty = ttyRef.current as ReturnType<typeof ctx.createTty>;
  const [session, setSession] = React.useState<Session>(null);
  const [status, setStatus] = React.useState<Status>('loading');

  function closeDocument() {
    ctx.closeTab(doc);
  }

  React.useEffect(() => {
    function initTty(session, mode?: ParticipantMode) {
      tracer.startActiveSpan(
        'initTTY',
        undefined, // SpanOptions
        context.active(),
        span => {
          const tty = ctx.createTty(session, mode);

          // subscribe to tty events to handle connect/disconnects events
          tty.on(TermEvent.CLOSE, () => ctx.closeTab(doc));

          tty.on(TermEvent.CONN_CLOSE, () =>
            ctx.updateSshDocument(doc.id, { status: 'disconnected' })
          );

          tty.on(TermEvent.SESSION, payload => {
            const data = JSON.parse(payload);
            data.session.kind = 'ssh';
            data.session.resourceName = data.session.server_hostname;
            setSession(data.session);
            handleTtyConnect(ctx, data.session, doc.id);
          });

          // assign tty reference so it can be passed down to xterm
          ttyRef.current = tty;
          setSession(session);
          setStatus('initialized');
          span.end();
        }
      );
    }
    initTty(
      {
        login,
        serverId,
        clusterId,
        sid,
      },
      mode
    );

    function teardownTty() {
      ttyRef.current && ttyRef.current.removeAllListeners();
    }

    return teardownTty;
  }, []);

  return {
    tty,
    status,
    session,
    closeDocument,
  };
}

function handleTtyConnect(
  ctx: ConsoleContext,
  session: SessionMetadata,
  docId: number
) {
  const {
    resourceName,
    login,
    id: sid,
    cluster_name: clusterId,
    server_id: serverId,
    created,
  } = session;

  const url = cfg.getSshSessionRoute({ sid, clusterId });
  const createdDate = new Date(created);
  ctx.updateSshDocument(docId, {
    title: `${login}@${resourceName}`,
    status: 'connected',
    url,
    serverId,
    created: createdDate,
    login,
    sid,
    clusterId,
  });

  ctx.gotoTab({ url });
}

type Status = 'initialized' | 'loading';
