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

import React, { useRef, useEffect } from 'react';
import { useTheme } from 'styled-components';

import { Indicator, Box } from 'design';

import {
  FileTransferActionBar,
  FileTransfer,
  FileTransferRequests,
  FileTransferContextProvider,
} from 'shared/components/FileTransfer';

import * as stores from 'teleport/Console/stores';

import AuthnDialog from 'teleport/components/AuthnDialog';
import useWebAuthn from 'teleport/lib/useWebAuthn';

import Document from '../Document';

import { Terminal, TerminalRef } from './Terminal';
import useSshSession from './useSshSession';
import { useFileTransfer } from './useFileTransfer';

export default function DocumentSshWrapper(props: PropTypes) {
  return (
    <FileTransferContextProvider>
      <DocumentSsh {...props} />
    </FileTransferContextProvider>
  );
}

function DocumentSsh({ doc, visible }: PropTypes) {
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument, session } = useSshSession(doc);
  const webauthn = useWebAuthn(tty);
  const {
    getMfaResponseAttempt,
    getDownloader,
    getUploader,
    fileTransferRequests,
  } = useFileTransfer(tty, session, doc, webauthn.addMfaToScpUrls);
  const theme = useTheme();

  function handleCloseFileTransfer() {
    terminalRef.current?.focus();
  }

  function handleFileTransferDecision(requestId: string, approve: boolean) {
    tty.approveFileTransferRequest(requestId, approve);
  }

  useEffect(() => {
    // when switching tabs or closing tabs, focus on visible terminal
    terminalRef.current?.focus();
  }, [visible, webauthn.requested]);

  const terminal = (
    <Terminal
      ref={terminalRef}
      tty={tty}
      fontFamily={theme.fonts.mono}
      theme={theme.colors.terminal}
    />
  );

  return (
    <Document visible={visible} flexDirection="column">
      <FileTransferActionBar isConnected={doc.status === 'connected'} />
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
      {status === 'initialized' && terminal}
      <FileTransfer
        FileTransferRequestsComponent={
          <FileTransferRequests
            onDeny={handleFileTransferDecision}
            onApprove={handleFileTransferDecision}
            requests={fileTransferRequests}
          />
        }
        beforeClose={() =>
          window.confirm('Are you sure you want to cancel file transfers?')
        }
        errorText={
          getMfaResponseAttempt.status === 'failed'
            ? getMfaResponseAttempt.statusText
            : null
        }
        afterClose={handleCloseFileTransfer}
        transferHandlers={{
          getDownloader,
          getUploader,
        }}
      />
    </Document>
  );
}

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
}
