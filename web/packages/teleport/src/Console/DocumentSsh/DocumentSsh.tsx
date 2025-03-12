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

import { Box, ButtonPrimary, Flex, H3, H4, Indicator, P2 } from 'design';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';
import { ShieldCheck } from 'design/Icon';
import { P } from 'design/Text/Text';
import {
  FileTransfer,
  FileTransferActionBar,
  FileTransferContextProvider,
  FileTransferRequests,
  useFileTransferContext,
} from 'shared/components/FileTransfer';
import { TerminalSearch } from 'shared/components/TerminalSearch';

import AuthnDialog from 'teleport/components/AuthnDialog';
import * as stores from 'teleport/Console/stores';
import { ItemStatus, StatusLight } from 'teleport/Discover/Shared';
import {
  MfaState,
  shouldShowMfaPrompt,
  useMfaEmitter,
} from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  ParticipantMode,
  SessionState,
  SessionStatus,
} from 'teleport/services/session';

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

function DocumentSsh({ doc, visible, mode }: PropTypes) {
  const ctx = useConsoleContext();
  const hasFileTransferAccess = ctx.storeUser.hasFileTransferAccess();
  const terminalRef = useRef<TerminalRef>();
  const { tty, status, closeDocument, session, sessionStatus } =
    useSshSession(doc);
  const [showSearch, setShowSearch] = useState(false);
  const [showMfaDialog, setShowMfaDialog] = useState(true);

  const mfa = useMfaEmitter(tty, {
    // The MFA requirement will be determined by whether we do/don't get
    // an mfa challenge over the event emitter at session start.
    isMfaRequired: false,
    req: {
      scope: MfaChallengeScope.USER_SESSION,
    },
  });
  const ft = useFileTransfer(tty, session, doc, mfa);
  const { openedDialog: ftOpenedDialog } = useFileTransferContext();

  const theme = useTheme();

  function handleCloseFileTransfer() {
    terminalRef.current?.focus();
  }

  function handleFileTransferDecision(requestId: string, approve: boolean) {
    tty.approveFileTransferRequest(requestId, approve);
  }

  useEffect(() => {
    // If an MFA attempt starts while switching tabs or closing tabs,
    // automatically focus on visible terminal.
    if (mfa.challenge) {
      terminalRef.current?.focus();
    }
  }, [visible, mfa.challenge]);

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
      {mode !== 'peer' && !!mode && (
        <Box
          width="100%"
          css={`
            background-color: ${theme.colors.interactive.tonal.primary[1]};
            border-bottom: ${theme.borders[1]}
              ${theme.colors.interactive.tonal.primary[2]};
          `}
          p={1}
        >
          {mode === 'moderator' && (
            <P2>
              You have joined this session as a <b>moderator</b>. You can watch
              and terminate the session, but cannot type.
            </P2>
          )}
          {mode === 'observer' && (
            <P2>
              You have joined this session as a <b>moderator</b>. You can watch
              this session, but not interact.
            </P2>
          )}
        </Box>
      )}
      <FileTransferActionBar
        hasAccess={hasFileTransferAccess}
        isConnected={doc.status === 'connected'}
      />
      {status === 'loading' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {sessionStatus?.state === SessionState.Pending ? (
        <WaitingRoomDialog
          sessionStatus={sessionStatus}
          mode={mode}
          mfa={mfa}
        />
      ) : (
        <AuthnDialog
          mfaState={mfa}
          onClose={() => {
            // Don't close the ssh doc if this is just a file transfer request.
            if (!ftOpenedDialog) {
              closeDocument();
            }
          }}
        />
      )}
      {status === 'initialized' && terminal}
    </Document>
  );
}

function WaitingRoomDialog({
  sessionStatus,
  mode,
  mfa,
}: {
  sessionStatus: SessionStatus;
  mode: ParticipantMode;
  mfa: MfaState;
}) {
  const ctx = useConsoleContext();

  const [showMfaDialog, setShowMfaDialog] = useState(false);

  return (
    <Dialog disableEscapeKeyDown={true} open={true}>
      <DialogHeader>
        <DialogTitle>Waiting for required participants...</DialogTitle>
      </DialogHeader>
      <P>
        One of the following policies must be fulfilled before the session can
        start:
      </P>
      <Flex
        flexDirection="column"
        gap={2}
        mt={3}
        maxHeight="420px"
        css={`
          overflow-y: auto;
          scrollbar-color: ${p => p.theme.colors.spotBackground[2]} transparent;
        `}
      >
        {sessionStatus.policyFulfillmentStatus.map(policy => (
          <Box
            key={policy.name}
            css={`
              border: ${props => props.theme.borders[1]}
                ${props => props.theme.colors.interactive.tonal.neutral[1]};
              border-radius: ${props => props.theme.radii[2]}px;
              background-color: ${props =>
                props.theme.colors.interactive.tonal.neutral[0]};
            `}
            px={3}
            py={2}
          >
            <Flex alignItems="center">
              <ShieldCheck mr={1} />
              <H3>Policy name: {policy.name}</H3>
            </Flex>
            <P mt={1}>
              Waiting for {policy.count - policy.satisfies.length} more
              participant
              {policy.count - policy.satisfies.length === 1
                ? ' that satisfies'
                : 's that satisfy'}{' '}
              this policy..
            </P>
            <H4 mt={2}>Current participants that satisfy this policy:</H4>
            <Flex flexDirection="column" mt={1}>
              {policy.satisfies.map(p => (
                <Flex key={p.user} flexDirection="row" alignItems="center">
                  <StatusLight status={ItemStatus.Success} />{' '}
                  <P>
                    {p.user}{' '}
                    {ctx.storeUser.getUsername() === p.user ? <b>(me)</b> : ''}
                  </P>
                </Flex>
              ))}
              {policy.satisfies.length === 0 && 'None'}
            </Flex>
          </Box>
        ))}
      </Flex>
      <H4 mt={3}>All participants currently in this session:</H4>
      {sessionStatus.parties.map(p => (
        <Flex key={p.user} flexDirection="row" alignItems="center">
          <StatusLight status={ItemStatus.Success} />{' '}
          <P>
            {p.user} ({p.mode}){' '}
            {ctx.storeUser.getUsername() === p.user ? <b>(me)</b> : ''}
          </P>
        </Flex>
      ))}

      {shouldShowMfaPrompt(mfa) && (
        <ButtonPrimary
          onClick={() => setShowMfaDialog(true)}
          mt={4}
          width="180px"
        >
          Join this session
        </ButtonPrimary>
      )}

      {showMfaDialog && (
        <AuthnDialog
          mfaState={mfa}
          onClose={() => {
            setShowMfaDialog(false);
          }}
        />
      )}
    </Dialog>
  );
}

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
  mode: ParticipantMode;
}

const mockSessionStatus: SessionStatus = {
  state: SessionState.Pending,
  parties: [
    {
      user: 'joe',
      mode: 'moderator',
    },
    {
      user: 'bob',
      mode: 'moderator',
    },
    {
      user: 'moderated',
      mode: 'peer',
    },
    {
      user: 'michael',
      mode: 'observer',
    },
  ],
  policyFulfillmentStatus: [
    {
      name: 'Require 2 moderators',
      count: 2,
      satisfies: [
        {
          user: 'joe',
          mode: 'moderator',
        },
      ],
    },
    {
      name: 'Require 4 moderators',
      count: 4,
      satisfies: [
        {
          user: 'joe',
          mode: 'moderator',
        },
        {
          user: 'bob',
          mode: 'moderator',
        },
      ],
    },
  ],
};
