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
import { useRef, useEffect } from 'react';

import { useTheme } from 'styled-components';
import { Box, Indicator } from 'design';

import * as stores from 'teleport/Console/stores/types';
import { Terminal, TerminalRef } from 'teleport/Console/DocumentSsh/Terminal';
import { useMfa } from 'teleport/lib/useMfa';

import Document from 'teleport/Console/Document';
import AuthnDialog from 'teleport/components/AuthnDialog';

import { useDbSession } from './useDbSession';
import { ConnectDialog } from './ConnectDialog';

type Props = {
  visible: boolean;
  doc: stores.DocumentDb;
};

export function DocumentDb({ doc, visible }: Props) {
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument, sendDbConnectData } = useDbSession(doc);
  const mfa = useMfa(tty);
  useEffect(() => {
    // when switching tabs or closing tabs, focus on visible terminal
    terminalRef.current?.focus();
  }, [visible, mfa.requested, status]);
  const theme = useTheme();

  return (
    <Document visible={visible} flexDirection="column">
      {status === 'loading' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {mfa.requested && <AuthnDialog mfa={mfa} onCancel={closeDocument} />}

      {status === 'waiting' && (
        <ConnectDialog
          clusterId={doc.clusterId}
          serviceName={doc.name}
          onConnect={sendDbConnectData}
          onClose={closeDocument}
        />
      )}
      {status !== 'loading' && (
        <Terminal
          ref={terminalRef}
          tty={tty}
          fontFamily={theme.fonts.mono}
          theme={theme.colors.terminal}
          convertEol
          // TODO(gabrielcorado): remove this once the server can properly handle it.
          disableCtrlC={true}
        />
      )}
    </Document>
  );
}
