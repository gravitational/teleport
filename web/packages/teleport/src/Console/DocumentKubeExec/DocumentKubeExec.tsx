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
import { useEffect, useRef } from 'react';
import { useTheme } from 'styled-components';

import { Box, Indicator } from 'design';

import AuthnDialog from 'teleport/components/AuthnDialog';
import Document from 'teleport/Console/Document';
import useKubeExecSession from 'teleport/Console/DocumentKubeExec/useKubeExecSession';
import { Terminal, TerminalRef } from 'teleport/Console/DocumentSsh/Terminal';
import * as stores from 'teleport/Console/stores/types';
import { useMfaEmitter } from 'teleport/lib/useMfa';

import KubeExecData from './KubeExecDataDialog';

type Props = {
  visible: boolean;
  doc: stores.DocumentKubeExec;
};

export default function DocumentKubeExec({ doc, visible }: Props) {
  const terminalRef = useRef<TerminalRef>(undefined);
  const { tty, status, closeDocument, sendKubeExecData } =
    useKubeExecSession(doc);
  const mfa = useMfaEmitter(tty);
  useEffect(() => {
    // when switching tabs, closing tabs, or
    // when the pod information modal is dismissed
    // focus the terminal
    if (status === 'initialized') {
      terminalRef.current?.focus();
    }
  }, [visible, mfa.challenge, status]);
  const theme = useTheme();

  const terminal = (
    <Terminal
      ref={terminalRef}
      tty={tty}
      fontFamily={theme.fonts.mono}
      theme={theme.colors.terminal}
      convertEol
      disableAutoFocus={true}
    />
  );

  return (
    <Document visible={visible} flexDirection="column">
      {status === 'loading' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      <AuthnDialog mfaState={mfa} onClose={closeDocument} />

      {status === 'waiting-for-exec-data' && (
        <KubeExecData onExec={sendKubeExecData} onClose={closeDocument} />
      )}
      {status !== 'loading' && terminal}
    </Document>
  );
}
