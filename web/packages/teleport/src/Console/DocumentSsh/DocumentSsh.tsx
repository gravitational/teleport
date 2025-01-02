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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useTheme } from 'styled-components';

import { Box, Indicator } from 'design';
import {
  FileTransfer,
  FileTransferActionBar,
  FileTransferContextProvider,
  FileTransferRequests,
} from 'shared/components/FileTransfer';
import { TerminalSearch } from 'shared/components/TerminalSearch';

import AuthnDialog from 'teleport/components/AuthnDialog';
import * as stores from 'teleport/Console/stores';
import useWebAuthn from 'teleport/lib/useWebAuthn';

import { useConsoleContext } from '../consoleContextProvider';
import Document from '../Document';
import { Terminal, TerminalRef } from './Terminal';
import { useFileTransfer } from './useFileTransfer';
import useSshSession from './useSshSession';

export default function DocumentSshWrapper(props: PropTypes) {
  return (
    <FileTransferContextProvider>
      <DocumentSsh {...props} />
    </FileTransferContextProvider>
  );
}

function DocumentSsh({ doc, visible }: PropTypes) {
  const ctx = useConsoleContext();
  const hasFileTransferAccess = ctx.storeUser.hasFileTransferAccess();
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument, session } = useSshSession(doc);
  const [showSearch, setShowSearch] = useState(false);
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

  const onSearchClose = useCallback(() => {
    setShowSearch(false);
  }, []);

  const onSearchOpen = useCallback(() => {
    setShowSearch(true);
  }, []);

  const isSearchKeyboardEvent = useCallback((e: KeyboardEvent) => {
    return (e.metaKey || e.ctrlKey) && e.key === 'f';
  }, []);

  const terminal = (
    <Terminal
      ref={terminalRef}
      tty={tty}
      fontFamily={theme.fonts.mono}
      theme={theme.colors.terminal}
      terminalAddons={ref => (
        <>
          <TerminalSearch
            show={showSearch}
            onClose={onSearchClose}
            onOpen={onSearchOpen}
            terminalSearcher={ref}
            isSearchKeyboardEvent={isSearchKeyboardEvent}
          />
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
        </>
      )}
    />
  );

  return (
    <Document visible={visible} flexDirection="column">
      <FileTransferActionBar
        hasAccess={hasFileTransferAccess}
        isConnected={doc.status === 'connected'}
      />
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
    </Document>
  );
}

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
}
