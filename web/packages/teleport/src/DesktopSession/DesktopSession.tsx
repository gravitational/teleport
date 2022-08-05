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

import React, { PropsWithChildren } from 'react';
import { Indicator, Box, Text, Flex, ButtonSecondary } from 'design';
import { Danger, Warning } from 'design/Alert';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import AuthnDialog from 'teleport/components/AuthnDialog';

import useDesktopSession, { State } from './useDesktopSession';
import TopBar from './TopBar';

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
    clipboardState,
    fetchAttempt,
    tdpConnection,
    disconnected,
    wsConnection,
    setTdpConnection,
  } = props;

  const clipboardProcessing =
    clipboardState.enabled && clipboardState.permission.state === 'prompt';

  const processing =
    fetchAttempt.status === 'processing' ||
    tdpConnection.status === 'processing' ||
    clipboardProcessing;

  // onDialogClose is called when a user
  // dismisses a non-fatal error dialog.
  const onDialogClose = () => {
    // This setTdpConnection call will cause the useEffect below
    // to calculate the errorDialog state.
    setTdpConnection(prevState => {
      // onDialogClose should only be called when
      // the user dismisses a non-fatal error dialog,
      // and prevState.status === '' means non-fatal
      // error dialog, so the below if statement should
      // always be true.
      if (prevState.status === '') {
        // If prevState.status was a non-fatal error,
        // we assume that the TDP connection remains open.
        return { status: 'success' };
      }
      return prevState;
    });
  };

  const computeErrorDialog = () => {
    const clipboardError = clipboardState.enabled && clipboardState.errorText;

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
    } else if (clipboardError) {
      errorText = clipboardState.errorText || 'clipboard sharing failed';
    } else if (unknownConnectionError) {
      errorText = 'Session disconnected for an unknown reason.';
    }
    const open = errorText !== '';
    const fatal = tdpConnection.status !== '';

    return { open, text: errorText, fatal };
  };

  const errorDialog = computeErrorDialog();

  if (errorDialog.open) {
    return (
      <Session {...props}>
        <Dialog
          dialogCss={() => ({ width: '484px' })}
          onClose={onDialogClose}
          open={errorDialog.open}
        >
          <DialogHeader style={{ flexDirection: 'column' }}>
            {errorDialog.fatal && <DialogTitle>Fatal Error</DialogTitle>}
            {!errorDialog.fatal && (
              <DialogTitle>Unsupported Action</DialogTitle>
            )}
          </DialogHeader>
          <DialogContent>
            {errorDialog.fatal && (
              <>
                <Danger children={<>{errorDialog.text}</>} />
                Refresh the page to try again.
              </>
            )}

            {!errorDialog.fatal && (
              <Warning my={2} children={errorDialog.text} />
            )}
          </DialogContent>
          <DialogFooter>
            {!errorDialog.fatal && (
              <ButtonSecondary size="large" width="30%" onClick={onDialogClose}>
                Dismiss
              </ButtonSecondary>
            )}
            {errorDialog.fatal && (
              <ButtonSecondary
                size="large"
                width="30%"
                onClick={() => {
                  window.location.reload();
                }}
              >
                Refresh
              </ButtonSecondary>
            )}
          </DialogFooter>
        </Dialog>
      </Session>
    );
  }

  if (disconnected) {
    return (
      <Session {...props}>
        <Box textAlign="center" m={10}>
          <Text>Session successfully disconnected</Text>
        </Box>
      </Session>
    );
  }

  if (processing) {
    return (
      <Session {...props}>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Session>
    );
  }

  return <Session {...props}></Session>;
}

function Session(props: PropsWithChildren<State>) {
  const {
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
    setDisconnected,
    webauthn,
    tdpClient,
    username,
    hostname,
    clipboardState,
    setClipboardState,
    canShareDirectory,
    isSharingDirectory,
    setIsSharingDirectory,
    onPngFrame,
    onClipboardData,
    onTdpError,
    onWsClose,
    onWsOpen,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
    onContextMenu,
    onMouseEnter,
    windowOnFocus,
  } = props;

  const clipboardSharingActive =
    clipboardState.enabled && clipboardState.permission.state === 'granted';
  const clipboardSuccess =
    !clipboardState.enabled ||
    (clipboardState.enabled &&
      clipboardState.permission.state === 'granted' &&
      clipboardState.errorText === '');

  const showCanvas =
    fetchAttempt.status === 'success' &&
    (tdpConnection.status === 'success' || tdpConnection.status === '') &&
    wsConnection === 'open' &&
    !disconnected &&
    clipboardSuccess;

  const onShareDirectory = () => {
    window
      .showDirectoryPicker()
      .then(sharedDirHandle => {
        setIsSharingDirectory(true);
        tdpClient.addSharedDirectory(sharedDirHandle);
        tdpClient.sendSharedDirectoryAnnounce();
      })
      .catch(() => {
        setIsSharingDirectory(false);
      });
  };

  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          setDisconnected(true);
          setClipboardState(prevState => ({
            ...prevState,
            enabled: false,
          }));
          setIsSharingDirectory(false);
          tdpClient.nuke();
        }}
        userHost={`${username}@${hostname}`}
        clipboardSharingEnabled={clipboardSharingActive}
        canShareDirectory={canShareDirectory}
        isSharingDirectory={isSharingDirectory}
        onShareDirectory={onShareDirectory}
      />

      {props.children}

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

      {/* TdpClientCanvas should always be present in the DOM so that it calls
          tdpClient.init() and initializes the required tdpClient event listeners,
          both of which are needed for this component's state to properly respond to
          initialization events. */}
      <TdpClientCanvas
        style={{
          display: showCanvas ? 'flex' : 'none',
          flex: 1, // ensures the canvas fills available screen space
        }}
        tdpCli={tdpClient}
        tdpCliOnPngFrame={onPngFrame}
        tdpCliOnClipboardData={onClipboardData}
        tdpCliOnTdpError={onTdpError}
        tdpCliOnWsClose={onWsClose}
        tdpCliOnWsOpen={onWsOpen}
        onKeyDown={onKeyDown}
        onKeyUp={onKeyUp}
        onMouseMove={onMouseMove}
        onMouseDown={onMouseDown}
        onMouseUp={onMouseUp}
        onMouseWheelScroll={onMouseWheelScroll}
        onContextMenu={onContextMenu}
        onMouseEnter={onMouseEnter}
        windowOnFocus={windowOnFocus}
      />
    </Flex>
  );
}
