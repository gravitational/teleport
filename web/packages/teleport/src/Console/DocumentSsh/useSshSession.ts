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

          tty.on(TermEvent.LATENCY, payload => {
            const stats = JSON.parse(payload);
            ctx.updateSshDocument(doc.id, {
              latency: {
                client: stats.ws,
                server: stats.ssh,
              },
            });
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
        kind: 'ssh',
        login,
        serverId,
        clusterId,
        sid,
        kubeExec: doc.kubeExec,
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
