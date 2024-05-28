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
import React, { useRef, useEffect, useState } from 'react';

import { useTheme } from 'styled-components';
import { Box, Indicator } from 'design';

import * as stores from 'teleport/Console/stores/types';
import { Terminal, TerminalRef } from 'teleport/Console/DocumentSsh/Terminal';
import useWebAuthn from 'teleport/lib/useWebAuthn';
import useKubeExecSession from 'teleport/Console/DocumentKubeExec/useKubeExecSession';

import Document from 'teleport/Console/Document';
import AuthnDialog from 'teleport/components/AuthnDialog';

import KubeExecData from './KubeExecDataDialog';

type Props = {
  visible: boolean;
  doc: stores.DocumentKubeExec;
};

export default function DocumentKubeExec({ doc, visible }: Props) {
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument } = useKubeExecSession(doc);
  const [sentData, setSentData] = useState(false);
  const webauthn = useWebAuthn(tty);
  useEffect(() => {
    // when switching tabs or closing tabs, focus on visible terminal
    terminalRef.current?.focus();
  }, [visible, webauthn.requested]);
  const theme = useTheme();

  const terminal = (
    <Terminal
      ref={terminalRef}
      tty={tty}
      fontFamily={theme.fonts.mono}
      theme={theme.colors.terminal}
      assistEnabled={false}
      convertEol={!doc.isInteractive}
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

      {status === 'initialized' && !sentData && (
        <KubeExecData
          onExec={(namespace, pod, container, command, isInteractive) => {
            tty.sendKubeExecData({
              kubeCluster: doc.kubeCluster,
              namespace,
              pod,
              container,
              command,
              isInteractive,
            });
            setSentData(true);
          }}
          onClose={closeDocument}
        />
      )}
      {status !== 'loading' && terminal}
    </Document>
  );
}
