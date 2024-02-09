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

import React, { useState, useEffect } from 'react';
import { Indicator, Box, Flex, ButtonSecondary, ButtonPrimary } from 'design';
import { Info } from 'design/Alert';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import AuthnDialog from 'teleport/components/AuthnDialog';

import useDesktopSession, {
  directorySharingPossible,
  isSharingClipboard,
  isSharingDirectory,
} from './useDesktopSession';
import TopBar from './TopBar';

import type { PropsWithChildren } from 'react';

import type { State } from './useDesktopSession';

export default function Container() {
  const state = useDesktopSession();
  return <DesktopSession {...state} />;
}

declare global {
  interface Window {
    showDirectoryPicker: () => Promise<FileSystemDirectoryHandle>;
  }
}

const invalidStateMessage =
  'The application has detected an invalid internal application state. \
            Please file a bug report for this issue at \
            https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md';

export function DesktopSession(props: State) {
  const {
    fetchAttempt,
    tdpConnection,
    wsConnection,
    showAnotherSessionActiveDialog,
    setShowAnotherSessionActiveDialog,
  } = props;

  const [state, setState] = useState<{
    screen: 'processing' | 'anotherSessionActive' | 'canvas' | 'alert dialog';
    alertMessage?: string;
  }>({
    screen: 'processing',
  });

  useEffect(() => {
    setState(prevState => {
      // We always want to show the user the first alert that caused the session to fail/end,
      // so if we're already showing an alert, don't change the screen.
      //
      // This allows us to track the various pieces of the state independently and always display
      // the vital information to the user. For example, we can track the TDP connection status
      // and the websocket connection status separately throughout the codebase. If the TDP connection
      // fails, and then the websocket closes, we want to show the TDP connection error to the user,
      // not the websocket closing. But if the websocket closes unexpectedly before a TDP message telling
      // us why, we want to show the websocket closing message to the user.
      if (prevState.screen === 'alert dialog') {
        return prevState;
      } else {
        // Otherwise, calculate screen state:
        const showAnotherSessionActive = showAnotherSessionActiveDialog;
        const showAlert =
          fetchAttempt.status === 'failed' || // Fetch attempt failed
          tdpConnection.status === 'failed' || // TDP connection failed
          tdpConnection.status === '' || // TDP connection ended gracefully server-side
          wsConnection.status === 'closed'; // Websocket closed (could mean client side graceful close or unexpected close, the message will tell us which)
        const processing =
          fetchAttempt.status === 'processing' ||
          tdpConnection.status === 'processing';

        if (showAnotherSessionActive) {
          return { screen: 'anotherSessionActive' };
        } else if (showAlert) {
          let message = '';
          if (fetchAttempt.status === 'failed') {
            message = fetchAttempt.statusText || 'fetch attempt failed';
          } else if (tdpConnection.status === 'failed') {
            message = tdpConnection.statusText || 'TDP connection failed';
          } else if (tdpConnection.status === '') {
            message =
              tdpConnection.statusText || 'TDP connection ended gracefully';
          } else if (wsConnection.status === 'closed') {
            message =
              wsConnection.message ||
              'websocket disconnected for an unknown reason';
          } else {
            message = invalidStateMessage;
          }
          return { screen: 'alert dialog', alertMessage: message };
        } else if (processing) {
          return { screen: 'processing' };
        } else {
          return { screen: 'canvas' };
        }
      }
    });
  }, [
    fetchAttempt,
    tdpConnection,
    wsConnection,
    showAnotherSessionActiveDialog,
  ]);

  if (state.screen === 'alert dialog') {
    return (
      <Session {...props} clientShouldConnect={false} displayCanvas={false}>
        <Dialog dialogCss={() => ({ width: '484px' })} open={true}>
          <DialogHeader style={{ flexDirection: 'column' }}>
            <DialogTitle>Disconnected</DialogTitle>
          </DialogHeader>
          <DialogContent>
            <>
              <Info
                children={<>{state.alertMessage || invalidStateMessage}</>}
              />
              Refresh the page to reconnect.
            </>
          </DialogContent>
          <DialogFooter>
            <ButtonSecondary
              size="large"
              width="30%"
              onClick={() => {
                window.location.reload();
              }}
            >
              Refresh
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      </Session>
    );
  } else if (state.screen === 'anotherSessionActive') {
    // Don't start the TDP connection until the user confirms they're ok
    // with potentially killing another user's connection.
    const shouldConnect = false;

    return (
      <Session
        {...props}
        clientShouldConnect={shouldConnect}
        displayCanvas={false}
      >
        <Dialog
          dialogCss={() => ({ width: '484px' })}
          onClose={() => {}}
          open={true}
        >
          <DialogHeader style={{ flexDirection: 'column' }}>
            <DialogTitle>Another Session Is Active</DialogTitle>
          </DialogHeader>
          <DialogContent>
            This desktop has an active session, connecting to it may close the
            other session. Do you wish to continue?
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr={3}
              onClick={() => {
                window.close();
              }}
            >
              Abort
            </ButtonPrimary>
            <ButtonSecondary
              onClick={() => {
                setShowAnotherSessionActiveDialog(false);
              }}
            >
              Continue
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      </Session>
    );
  } else if (state.screen === 'processing') {
    // We don't know whether another session for this desktop is active while the
    // fetchAttempt is still processing, so hold off on starting a TDP connection
    // until that information is available.
    const shouldConnect = fetchAttempt.status !== 'processing';

    return (
      <Session
        {...props}
        clientShouldConnect={shouldConnect}
        displayCanvas={false}
      >
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Session>
    );
  }

  return <Session {...props} clientShouldConnect={true} displayCanvas={true} />;
}

function Session({
  webauthn,
  tdpClient,
  username,
  hostname,
  setClipboardSharingState,
  directorySharingState,
  setDirectorySharingState,
  clientOnPngFrame,
  clientOnBitmapFrame,
  clientOnClientScreenSpec,
  clientOnClipboardData,
  clientOnTdpError,
  clientOnTdpWarning,
  clientOnTdpInfo,
  clientOnWsClose,
  clientOnWsOpen,
  canvasOnKeyDown,
  canvasOnKeyUp,
  canvasOnFocusOut,
  canvasOnMouseMove,
  canvasOnMouseDown,
  canvasOnMouseUp,
  canvasOnMouseWheelScroll,
  canvasOnContextMenu,
  clientShouldConnect,
  clientScreenSpecToRequest,
  displayCanvas,
  clipboardSharingState,
  onShareDirectory,
  warnings,
  onRemoveWarning,
  children,
}: PropsWithChildren<Props>) {
  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          setClipboardSharingState(prevState => ({
            ...prevState,
            isSharing: false,
          }));
          setDirectorySharingState(prevState => ({
            ...prevState,
            isSharing: false,
          }));
          tdpClient.shutdown();
        }}
        userHost={`${username}@${hostname}`}
        canShareDirectory={directorySharingPossible(directorySharingState)}
        isSharingDirectory={isSharingDirectory(directorySharingState)}
        isSharingClipboard={isSharingClipboard(clipboardSharingState)}
        onShareDirectory={onShareDirectory}
        warnings={warnings}
        onRemoveWarning={onRemoveWarning}
      />

      {children}

      {webauthn.requested && (
        <AuthnDialog
          onContinue={webauthn.authenticate}
          onCancel={() => {
            webauthn.setState(prevState => {
              return {
                ...prevState,
                errorText:
                  'This session requires multi factor authentication to continue. Please hit "Retry" and follow the prompts given by your browser to complete authentication.',
              };
            });
          }}
          errorText={webauthn.errorText}
        />
      )}

      <TdpClientCanvas
        style={{
          display: displayCanvas ? 'flex' : 'none',
        }}
        client={tdpClient}
        clientShouldConnect={clientShouldConnect}
        clientScreenSpecToRequest={clientScreenSpecToRequest}
        clientOnPngFrame={clientOnPngFrame}
        clientOnBmpFrame={clientOnBitmapFrame}
        clientOnClientScreenSpec={clientOnClientScreenSpec}
        clientOnClipboardData={clientOnClipboardData}
        clientOnTdpError={clientOnTdpError}
        clientOnTdpWarning={clientOnTdpWarning}
        clientOnTdpInfo={clientOnTdpInfo}
        clientOnWsClose={clientOnWsClose}
        clientOnWsOpen={clientOnWsOpen}
        canvasOnKeyDown={canvasOnKeyDown}
        canvasOnKeyUp={canvasOnKeyUp}
        canvasOnFocusOut={canvasOnFocusOut}
        canvasOnMouseMove={canvasOnMouseMove}
        canvasOnMouseDown={canvasOnMouseDown}
        canvasOnMouseUp={canvasOnMouseUp}
        canvasOnMouseWheelScroll={canvasOnMouseWheelScroll}
        canvasOnContextMenu={canvasOnContextMenu}
      />
    </Flex>
  );
}

type Props = State & {
  // Determines whether the tdp client that's passed to the TdpClientCanvas
  // should connect to the server.
  clientShouldConnect: boolean;
  displayCanvas: boolean;
};
