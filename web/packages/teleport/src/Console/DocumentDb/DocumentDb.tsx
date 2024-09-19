
/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import React, { useRef, useEffect, useCallback } from 'react';

import { useTheme } from 'styled-components';
import { Box, Indicator } from 'design';

import * as stores from 'teleport/Console/stores/types';
import { Terminal, TerminalRef } from 'teleport/Console/DocumentSsh/Terminal';
import useWebAuthn from 'teleport/lib/useWebAuthn';
import useDbSession from './useDbSession';
import ConnectDialog from './ConnectDialog';

import Document from 'teleport/Console/Document';
import AuthnDialog from 'teleport/components/AuthnDialog';

import { useTeleport } from 'teleport';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';

type Props = {
  visible: boolean;
  doc: stores.DocumentDb;
};

export default function DocumentDb({ doc, visible }: Props) {
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument, sendConnectData } = useDbSession(doc);
  const webauthn = useWebAuthn(tty);
  useEffect(() => {
    // when switching tabs or closing tabs, focus on visible terminal
    terminalRef.current?.focus();
  }, [visible, webauthn.requested, status]);
  const theme = useTheme();

  // TODO: should we introduce a different API instead of using unified resources?
  const ctx = useTeleport();
  const {
    fetch: unifiedFetch,
    resources,
    // attempt: unifiedFetchAttempt,
    // clear,
  } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (_, signal) => {
        const response = await ctx.resourceService.fetchUnifiedResources(
          doc.clusterId,
          {
            // search: agentFilter.search,
            query: `name == "${doc.name}"`,
            sort: { fieldName: 'name', dir: 'ASC'},
            // kinds: ['db'],
            limit: 1,
          },
          signal
        );

        return { agents: response.agents };
      },
      [doc.clusterId, doc.name]
    )
  })

  useEffect(() => { unifiedFetch({ clear: true})}, [])

  const terminal = (
    <Terminal
      ref={terminalRef}
      tty={tty}
      fontFamily={theme.fonts.mono}
      theme={theme.colors.terminal}
      convertEol
    />
  );

  return (
    <Document visible={visible} flexDirection="column">
      {status === 'loading' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {webauthn.requested && (
        <AuthnDialog
          onContinue={webauthn.authenticate}
          onCancel={closeDocument}
          errorText={webauthn.errorText}
        />
      )}

      {status === 'waiting' && (
        <ConnectDialog db={resources[0]} onConnect={sendConnectData} onClose={closeDocument} />
      )}
      {status !== 'loading' && terminal}
    </Document>
  );
}

