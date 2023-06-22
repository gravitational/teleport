/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
import { Danger } from 'design/Alert';
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
    // closed it on purpose or registered a fatal tdp error.
    const unknownConnectionError =
      wsConnection === 'closed' &&
      !disconnected &&
      (tdpConnection.status === 'success' || tdpConnection.status === '');

    let errorText = '';
    if (fetchAttempt.status === 'failed') {
      errorText = fetchAttempt.statusText || 'fetch attempt failed';
    } else if (tdpConnection.status === 'failed') {
      errorText = tdpConnection.statusText || 'tdp connection failed';
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

    return { open, text: errorText };
  };

  const errorDialog = computeErrorDialog();

  if (errorDialog.open) {
    return (
      <Session {...props} connectTdpCli={false} displayCanvas={false}>
        <Dialog
          dialogCss={() => ({ width: '484px' })}
          onClose={onDialogClose}
          open={errorDialog.open}
        >
          <DialogHeader style={{ flexDirection: 'column' }}>
            <DialogTitle>Error</DialogTitle>
          </DialogHeader>
          <DialogContent>
            <>
              <Danger children={<>{errorDialog.text}</>} />
              Refresh the page to try again.
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
    const connectTdpCli = false;

    return (
      <Session {...props} connectTdpCli={connectTdpCli} displayCanvas={false}>
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
      <Session {...props} connectTdpCli={false} displayCanvas={false}>
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
    const connectTdpCli = fetchAttempt.status !== 'processing';

    return (
      <Session {...props} connectTdpCli={connectTdpCli} displayCanvas={false}>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Session>
    );
  }

  return <Session {...props} connectTdpCli={true} displayCanvas={true} />;
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
  onPngFrame,
  onBitmapFrame,
  onClipboardData,
  onTdpError,
  onTdpWarning,
  onWsClose,
  onWsOpen,
  onKeyDown,
  onKeyUp,
  onMouseMove,
  onMouseDown,
  onMouseUp,
  onMouseWheelScroll,
  onContextMenu,
  connectTdpCli,
  screenSpec,
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
          flex: 1, // ensures the canvas fills available screen space
        }}
        tdpCli={tdpClient}
        tdpCliConnect={connectTdpCli}
        tdpCliScreenSpec={screenSpec}
        tdpCliOnPngFrame={onPngFrame}
        tdpCliOnBmpFrame={onBitmapFrame}
        tdpCliOnClipboardData={onClipboardData}
        tdpCliOnTdpError={onTdpError}
        tdpCliOnTdpWarning={onTdpWarning}
        tdpCliOnWsClose={onWsClose}
        tdpCliOnWsOpen={onWsOpen}
        onKeyDown={onKeyDown}
        onKeyUp={onKeyUp}
        onMouseMove={onMouseMove}
        onMouseDown={onMouseDown}
        onMouseUp={onMouseUp}
        onMouseWheelScroll={onMouseWheelScroll}
        onContextMenu={onContextMenu}
      />
    </Flex>
  );
}

type Props = State & {
  // Determines whether the tdp client that's passed to the TdpClientCanvas
  // should be initialized.
  connectTdpCli: boolean;
  displayCanvas: boolean;
};
