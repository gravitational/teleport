
/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { DocumentDb } from 'teleport/Console/stores';

import type { Session, SessionMetadata, } from 'teleport/services/session';

const tracer = trace.getTracer('TTY');

export default function useDbSession(doc: DocumentDb) {
  const { clusterId, sid, name, dbUser, dbRoles, dbName } = doc;
  const ctx = useConsoleContext();
  const ttyRef = React.useRef<Tty>(null);
  const tty = ttyRef.current as ReturnType<typeof ctx.createTty>;
  const [session, setSession] = React.useState<Session>(null);
  const [status, setStatus] = React.useState<Status>('loading');

  function closeDocument() {
    ctx.closeTab(doc);
  }

  React.useEffect(() => {
    const session: any = {
      kind: 'db',
      resourceName: name,
      clusterId,
      sid,
    };
    tracer.startActiveSpan(
      'initTTY',
      undefined, // SpanOptions
      context.active(),
      span => {
        const tty = ctx.createTty(session);

        tty.on('open', () => {
          console.log("=====>>> SENDING INITIAL MESSAGE?", name, dbUser, dbRoles, dbName)
          tty.sendDbConnectData({ serviceName: name, dbUser, dbRoles, dbName })
        });

        // subscribe to tty events to handle connect/disconnects events
        tty.on(TermEvent.CLOSE, () => ctx.closeTab(doc));

        tty.on(TermEvent.CONN_CLOSE, () => {
          setStatus('disconnected');
          ctx.updateDbDocument(doc.id, { status: 'disconnected' });
        });

        tty.on(TermEvent.SESSION, payload => {
          const data = JSON.parse(payload);
          data.session.kind = 'db';
          setSession(data.session);
          handleTtyConnect(ctx, tty, doc.id, data.session);
        });

        // assign tty reference so it can be passed down to xterm
        ttyRef.current = tty;
        setSession(session);
        setStatus('initialized');
        span.end();
      }
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
  tty: Tty,
  docId: number,
  { id: sid, cluster_name: clusterId, created }: SessionMetadata,
) {
  const url = cfg.getDBExecUrl({ clusterId, sid });
  const createdDate = new Date(created);

  ctx.updateDbDocument(docId, {
    status: 'connected',
    url,
    created: createdDate,
    sid,
    clusterId,
  });

  ctx.gotoTab({ url });
}

export type Status =
  | 'loading'
  | 'initialized'
  | 'disconnected';
