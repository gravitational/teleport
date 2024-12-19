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

import { useCallback, useEffect, useRef, useState } from 'react';
import { useTheme } from 'styled-components';

import { Box, Indicator } from 'design';

import {
  FileTransfer,
  FileTransferActionBar,
  FileTransferContextProvider,
  FileTransferRequests,
} from 'shared/components/FileTransfer';
import { TerminalSearch } from 'shared/components/TerminalSearch';

import * as stores from 'teleport/Console/stores';

import AuthnDialog from 'teleport/components/AuthnDialog';
import { useMfa, useMfaTty } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import Document from '../Document';

import { useConsoleContext } from '../consoleContextProvider';

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

  const ttyMfa = useMfaTty(tty);
  const ftMfa = useMfa({
    isMfaRequired: ttyMfa.required,
    req: {
      scope: MfaChallengeScope.USER_SESSION,
    },
  });
  const ft = useFileTransfer(tty, session, doc, ftMfa);
  const theme = useTheme();

  function handleCloseFileTransfer() {
    terminalRef.current?.focus();
  }

  function handleFileTransferDecision(requestId: string, approve: boolean) {
    tty.approveFileTransferRequest(requestId, approve);
  }

  useEffect(() => {
    // when switching tabs or closing tabs, focus on visible terminal
    if (
      ttyMfa.attempt.status === 'processing' ||
      ftMfa.attempt.status === 'processing'
    ) {
      terminalRef.current?.focus();
    }
  }, [visible, ttyMfa.attempt.status, ftMfa.attempt.status]);

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
                requests={ft.fileTransferRequests}
              />
            }
            beforeClose={() =>
              window.confirm('Are you sure you want to cancel file transfers?')
            }
            afterClose={handleCloseFileTransfer}
            transferHandlers={{
              ...ft,
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
      <AuthnDialog
        {...ttyMfa}
        resetAttempt={() => {
          ttyMfa.resetAttempt();
          closeDocument();
        }}
      />
      <AuthnDialog {...ftMfa} />
      {status === 'initialized' && terminal}
    </Document>
  );
}

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
}
