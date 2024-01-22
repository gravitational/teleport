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

import React from 'react';
import {
  Indicator,
  Box,
  Text,
  Flex,
  ButtonSecondary,
  ButtonPrimary,
} from 'design';
import { Danger, Info } from 'design/Alert';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import AuthnDialog from 'teleport/components/AuthnDialog';

import useDesktopSession from './useDesktopSession';
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

export function DesktopSession(props: State) {
  const {
    fetchAttempt,
    tdpConnection,
    disconnected,
    wsConnection,
    setTdpConnection,
    showAnotherSessionActiveDialog,
    setShowAnotherSessionActiveDialog,
  } = props;

  const processing =
    fetchAttempt.status === 'processing' ||
    tdpConnection.status === 'processing';

  // onDialogClose is called when a user
  // dismisses a non-fatal error dialog.
  const onDialogClose = () => {
    // The following state-setting calls will
    // cause the useEffect below to calculate the
    // errorDialog state.

    setTdpConnection(prevState => {
      if (prevState.status === '') {
        // If prevState.status was a non-fatal error,
        // we assume that the TDP connection remains open.
        return { status: 'success' };
      }
      return prevState;
    });
  };

  const computeErrorDialog = () => {
    // Websocket is closed but we haven't
    // closed it on purpose or registered a fatal TDP error.
    const unknownConnectionError =
      wsConnection === 'closed' &&
      !disconnected &&
      (tdpConnection.status === 'success' || tdpConnection.status === '');

    let errorText = '';
    if (fetchAttempt.status === 'failed') {
      errorText = fetchAttempt.statusText || 'fetch attempt failed';
    } else if (tdpConnection.status === 'failed') {
      errorText = tdpConnection.statusText || 'TDP connection failed';
    } else if (tdpConnection.status === '') {
      errorText = tdpConnection.statusText || 'encountered a non-fatal error';
    } else if (unknownConnectionError) {
      errorText = 'Session disconnected for an unknown reason.';
    } else if (
      fetchAttempt.status === 'processing' &&
      tdpConnection.status === 'success'
    ) {
      errorText =
        'The application has detected an invalid internal application state. \
        Please file a bug report for this issue at \
        https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md';
    }
    const open = errorText !== '';

    return {
      open,
      text: errorText,
      isError: unknownConnectionError || errorText === 'RDP connection failed',
    };
  };

  const errorDialog = computeErrorDialog();
  const Alert = errorDialog.isError ? Danger : Info;

  if (errorDialog.open) {
    return (
      <Session {...props} clientShouldConnect={false} displayCanvas={false}>
        <Dialog
          dialogCss={() => ({ width: '484px' })}
          onClose={onDialogClose}
          open={errorDialog.open}
        >
          <DialogHeader style={{ flexDirection: 'column' }}>
            <DialogTitle>
              {errorDialog.isError ? 'Error' : 'Disconnected'}
            </DialogTitle>
          </DialogHeader>
          <DialogContent>
            <>
              <Alert children={<>{errorDialog.text}</>} />
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
  }

  if (showAnotherSessionActiveDialog) {
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
  }

  if (disconnected) {
    return (
      <Session {...props} clientShouldConnect={false} displayCanvas={false}>
        <Box textAlign="center" m={10}>
          <Text>Session successfully disconnected</Text>
        </Box>
      </Session>
    );
  }

  if (processing) {
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
  setDisconnected,
  webauthn,
  tdpClient,
  username,
  hostname,
  setClipboardSharingEnabled,
  directorySharingState,
  setDirectorySharingState,
  clientOnPngFrame,
  clientOnBitmapFrame,
  clientOnClientScreenSpec,
  clientOnClipboardData,
  clientOnTdpError,
  clientOnTdpWarning,
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
  clipboardSharingEnabled,
  onShareDirectory,
  warnings,
  onRemoveWarning,
  children,
}: PropsWithChildren<Props>) {
  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          setDisconnected(true);
          setClipboardSharingEnabled(false);
          setDirectorySharingState(prevState => ({
            ...prevState,
            isSharing: false,
          }));
          tdpClient.shutdown();
        }}
        userHost={`${username}@${hostname}`}
        canShareDirectory={directorySharingState.canShare}
        isSharingDirectory={directorySharingState.isSharing}
        clipboardSharingEnabled={clipboardSharingEnabled}
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
