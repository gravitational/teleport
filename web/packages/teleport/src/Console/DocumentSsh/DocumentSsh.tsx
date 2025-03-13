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

import {
  Box,
  Button,
  ButtonPrimary,
  Flex,
  H3,
  H4,
  Indicator,
  Input,
  P2,
} from 'design';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';
import { BroadcastSlash, Logout, ShieldCheck } from 'design/Icon';
import { H2, P } from 'design/Text/Text';
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
import Tty from 'teleport/lib/term/tty';
import { useMfaEmitter } from 'teleport/lib/useMfa';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  Participant,
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
    <Flex width="100%" height="100%">
      <Document visible={visible} flexDirection="column">
        {mode !== 'peer' &&
          !!mode &&
          sessionStatus?.state === SessionState.Running && (
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
                  You have joined this session as a <b>moderator</b>. You can
                  watch and terminate the session, but cannot type.
                </P2>
              )}
              {mode === 'observer' && (
                <P2>
                  You have joined this session as a <b>moderator</b>. You can
                  watch this session, but not interact.
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
        {sessionStatus?.state === SessionState.Pending && (
          <WaitingRoomDialog
            sessionStatus={sessionStatus}
            handleReadyToJoin={() => tty.readyToJoin()}
          />
        )}
        <AuthnDialog
          mfaState={mfa}
          onClose={() => {
            // Don't close the ssh doc if this is just a file transfer request.
            if (!ftOpenedDialog) {
              closeDocument();
            }
          }}
        />
        {status === 'initialized' && terminal}
      </Document>
      {sessionStatus?.state === SessionState.Running && (
        <PartiesList
          parties={sessionStatus?.parties || []}
          tty={tty}
          username={ctx.getStoreUser().username}
          sessionStatus={sessionStatus}
        />
      )}
    </Flex>
  );
}

function WaitingRoomDialog({
  handleReadyToJoin,
  sessionStatus,
}: {
  handleReadyToJoin(): void;
  sessionStatus: SessionStatus;
}) {
  const ctx = useConsoleContext();

  const [joining, setJoining] = useState(false);

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

      <ButtonPrimary
        disabled={joining}
        onClick={() => {
          setJoining(true);
          handleReadyToJoin();
        }}
        mt={4}
        width="180px"
      >
        {joining ? 'Joining...' : 'Join this session'}
      </ButtonPrimary>
    </Dialog>
  );
}

function PartiesList({
  parties,
  username,
  tty,
  sessionStatus,
}: {
  parties: Participant[];
  username: string;
  tty: Tty;
  sessionStatus: SessionStatus;
}) {
  const observers = parties.filter(p => p.mode === 'observer');
  const peers = parties.filter(p => p.mode === 'peer');
  const moderators = parties.filter(p => p.mode === 'moderator');

  const isModerator = !!moderators.find(m => m.user === username);

  const [chatText, setChatText] = useState('');

  return (
    <Flex
      backgroundColor="levels.surface"
      width="300px"
      height="100%"
      css={`
        border-left: ${props => props.theme.borders[2]}
          ${props => props.theme.colors.interactive.tonal.neutral[1]};
      `}
      flexDirection={'column'}
      p={3}
      justifyContent="space-between"
    >
      <Flex flexDirection="column" gap={1}>
        <H2 mb={2}>Participants</H2>
        {peers.length > 0 && (
          <Box>
            <H3>Peers</H3>
            <Flex flexDirection="column">
              {peers.map(p => (
                <Flex key={p.user} flexDirection="row" alignItems="center">
                  <StatusLight status={ItemStatus.Success} />{' '}
                  <P>
                    {p.user} {username === p.user ? <b>(me)</b> : ''}
                  </P>
                </Flex>
              ))}
            </Flex>
          </Box>
        )}
        {moderators.length > 0 && (
          <Box>
            <H3>Moderators</H3>
            <Flex flexDirection="column">
              {moderators.map(p => (
                <Flex key={p.user} flexDirection="row" alignItems="center">
                  <StatusLight status={ItemStatus.Success} />{' '}
                  <P>
                    {p.user} {username === p.user ? '(me)' : ''}
                  </P>
                </Flex>
              ))}
            </Flex>
          </Box>
        )}
        {observers.length > 0 && (
          <Box>
            <H3>Observers</H3>
            <Flex flexDirection="column">
              {observers.map(p => (
                <Flex key={p.user} flexDirection="row" alignItems="center">
                  <StatusLight status={ItemStatus.Success} />{' '}
                  <P>
                    {p.user} {username === p.user ? '(me)' : ''}
                  </P>
                </Flex>
              ))}
            </Flex>
          </Box>
        )}
      </Flex>

      <Flex flexDirection="column">
        <H2 mt={3}>Chatroom</H2>
        <Flex
          width="100%"
          height="300px"
          mt={1}
          css={`
            background-color: ${props =>
              props.theme.colors.interactive.tonal.neutral[1]};
            border: ${props => props.theme.borders[2]}
              ${props => props.theme.colors.interactive.tonal.neutral[2]};
            border-radius: ${props => props.theme.radii[2]}px;

            border-bottom: none;
            border-bottom-right-radius: 0px;
            border-bottom-left-radius: 0px;

            overflow-y: auto;
            scrollbar-color: ${p => p.theme.colors.spotBackground[2]}
              transparent;
          `}
          justifyContent="space-between"
          flexDirection="column"
        >
          <Flex
            px={2}
            py={1}
            pb={2}
            flexDirection="column-reverse"
            height="100%"
          >
            {sessionStatus?.chatLog.map((message, i) => (
              <Box key={i}>
                <P>{message}</P>
              </Box>
            ))}
          </Flex>
        </Flex>
        <form
          onSubmit={e => {
            e.preventDefault();
            tty.sendChatMessage(`[${username}]: ${chatText}`);
            setChatText('');
          }}
        >
          <Input
            value={chatText}
            onChange={e => setChatText(e.target.value)}
            placeholder="Type a message..."
            width="100%"
            css={`
              background-color: ${props =>
                props.theme.colors.interactive.tonal.neutral[0]};

              input {
                border-top-right-radius: 0px;
                border-top-left-radius: 0px;
              }
            `}
          />
        </form>
      </Flex>

      <Flex flexDirection="column" mb={1}>
        {isModerator && (
          <Button
            intent="danger"
            mb={2}
            onClick={() => tty.terminateModeratedSession()}
          >
            <BroadcastSlash size="small" mr={2} /> Terminate
          </Button>
        )}
        <Button onClick={() => tty.disconnect()}>
          <Logout size="small" mr={2} /> Disconnect
        </Button>
      </Flex>
    </Flex>
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
