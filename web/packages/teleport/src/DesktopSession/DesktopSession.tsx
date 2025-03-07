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

import { Box, ButtonPrimary, ButtonSecondary, Flex, Indicator } from 'design';
import { Info } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Attempt as AsyncAttempt } from 'shared/hooks/useAsync';
import { Attempt } from 'shared/hooks/useAttemptNext';

import AuthnDialog from 'teleport/components/AuthnDialog';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import { TdpClientCanvasRef } from 'teleport/components/TdpClientCanvas/TdpClientCanvas';
import { KeyboardHandler } from 'teleport/DesktopSession/KeyboardHandler';
import { ButtonState, ScrollAxis } from 'teleport/lib/tdp';
import { useListener } from 'teleport/lib/tdp/client';
import { MfaState, shouldShowMfaPrompt } from 'teleport/lib/useMfa';

import TopBar from './TopBar';
import useDesktopSession, {
  clipboardSharingMessage,
  defaultClipboardSharingState,
  defaultDirectorySharingState,
  directorySharingPossible,
  isSharingClipboard,
  isSharingDirectory,
  type State,
  type WebsocketAttempt,
} from './useDesktopSession';

export function DesktopSessionContainer() {
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
    mfa,
    tdpClient: client,
    username,
    hostname,
    directorySharingState,
    setDirectorySharingState,
    setInitialTdpConnectionSucceeded,
    onClipboardData,
    sendLocalClipboardToRemote,
    setWsConnection,
    clientScreenSpecToRequest,
    clipboardSharingState,
    setClipboardSharingState,
    onShareDirectory,
    alerts,
    onRemoveAlert,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    showAnotherSessionActiveDialog,
    addAlert,
    setTdpConnection,
  } = props;

  const [screenState, setScreenState] = useState<ScreenState>({
    screen: 'processing',
    canvasState: { shouldConnect: false, shouldDisplay: false },
  });
  const keyboardHandler = useRef(new KeyboardHandler());

  useEffect(() => {
    keyboardHandler.current = new KeyboardHandler();
    // On unmount, clear all the timeouts on the keyboardHandler.
    return () => {
      // eslint-disable-next-line react-hooks/exhaustive-deps
      keyboardHandler.current.dispose();
    };
  }, []);
  const { shouldConnect } = screenState.canvasState;
  // Call connect after all listeners have been registered
  useEffect(() => {
    if (client && shouldConnect) {
      client.connect(clientScreenSpecToRequest);
      return () => {
        client.shutdown();
      };
    }
  }, [client, shouldConnect]);

  // Calculate the next `ScreenState` whenever any of the constituent pieces of state change.
  useEffect(() => {
    setScreenState(prevState =>
      nextScreenState(
        prevState,
        fetchAttempt,
        tdpConnection,
        wsConnection,
        showAnotherSessionActiveDialog,
        mfa
      )
    );
  }, [
    fetchAttempt,
    tdpConnection,
    wsConnection,
    showAnotherSessionActiveDialog,
    mfa,
  ]);

  const tdpClientCanvasRef = useRef<TdpClientCanvasRef>(null);
  const onInitialTdpConnectionSucceeded = useCallback(() => {
    setInitialTdpConnectionSucceeded(() => {
      // TODO(gzdunek): This callback is a temporary fix for focusing the canvas.
      // Focus the canvas once we start rendering frames.
      // The timeout it a small hack, we should verify
      // what is the earliest moment we can focus the canvas.
      setTimeout(() => {
        tdpClientCanvasRef.current?.focus();
      }, 100);
    });
  }, [setInitialTdpConnectionSucceeded]);

  useListener(client?.onClipboardData, onClipboardData);

  const handleFatalError = useCallback(
    (error: Error) => {
      setDirectorySharingState(defaultDirectorySharingState);
      setClipboardSharingState(defaultClipboardSharingState);
      setTdpConnection({
        status: 'failed',
        statusText: error.message || error.toString(),
      });
    },
    [setClipboardSharingState, setDirectorySharingState, setTdpConnection]
  );
  useListener(client?.onError, handleFatalError);
  useListener(client?.onClientError, handleFatalError);

  const addWarning = useCallback(
    (warning: string) => {
      addAlert({
        content: warning,
        severity: 'warn',
      });
    },
    [addAlert]
  );
  useListener(client?.onWarning, addWarning);
  useListener(client?.onClientWarning, addWarning);

  useListener(
    client?.onInfo,
    useCallback(
      info => {
        addAlert({
          content: info,
          severity: 'info',
        });
      },
      [addAlert]
    )
  );

  useListener(
    client?.onWsClose,
    useCallback(
      statusText => {
        setWsConnection({ status: 'closed', statusText });
      },
      [setWsConnection]
    )
  );
  useListener(
    client?.onWsOpen,
    useCallback(() => {
      setWsConnection({ status: 'open' });
    }, [setWsConnection])
  );

  useListener(client?.onPointer, tdpClientCanvasRef.current?.setPointer);
  useListener(
    client?.onPngFrame,
    useCallback(
      frame => {
        onInitialTdpConnectionSucceeded();
        tdpClientCanvasRef.current?.renderPngFrame(frame);
      },
      [onInitialTdpConnectionSucceeded]
    )
  );
  useListener(
    client?.onBmpFrame,
    useCallback(
      frame => {
        onInitialTdpConnectionSucceeded();
        tdpClientCanvasRef.current?.renderBitmapFrame(frame);
      },
      [onInitialTdpConnectionSucceeded]
    )
  );
  useListener(client?.onReset, tdpClientCanvasRef.current?.clear);
  useListener(client?.onScreenSpec, tdpClientCanvasRef.current?.setResolution);

  function handleKeyDown(e: React.KeyboardEvent) {
    keyboardHandler.current.handleKeyboardEvent({
      cli: client,
      e: e.nativeEvent,
      state: ButtonState.DOWN,
    });

    // The key codes in the if clause below are those that have been empirically determined not
    // to count as transient activation events. According to the documentation, a keydown for
    // the Esc key and any "shortcut key reserved by the user agent" don't count as activation
    // events: https://developer.mozilla.org/en-US/docs/Web/Security/User_activation.
    if (e.key !== 'Meta' && e.key !== 'Alt' && e.key !== 'Escape') {
      // Opportunistically sync local clipboard to remote while
      // transient user activation is in effect.
      // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
      sendLocalClipboardToRemote();
    }
  }

  function handleKeyUp(e: React.KeyboardEvent) {
    keyboardHandler.current.handleKeyboardEvent({
      cli: client,
      e: e.nativeEvent,
      state: ButtonState.UP,
    });
  }

  function handleBlur() {
    keyboardHandler.current.onFocusOut();
  }

  function handleMouseMove(e: React.MouseEvent) {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    client.sendMouseMove(x, y);
  }

  function handleMouseDown(e: React.MouseEvent) {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      client.sendMouseButton(e.button, ButtonState.DOWN);
    }

    // Opportunistically sync local clipboard to remote while
    // transient user activation is in effect.
    // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
    sendLocalClipboardToRemote();
  }

  function handleMouseUp(e: React.MouseEvent) {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      client.sendMouseButton(e.button, ButtonState.UP);
    }
  }

  function handleMouseWheel(e: WheelEvent) {
    e.preventDefault();
    // We only support pixel scroll events, not line or page events.
    // https://developer.mozilla.org/en-US/docs/Web/API/WheelEvent/deltaMode
    if (e.deltaMode === WheelEvent.DOM_DELTA_PIXEL) {
      if (e.deltaX) {
        client.sendMouseWheelScroll(ScrollAxis.HORIZONTAL, -e.deltaX);
      }
      if (e.deltaY) {
        client.sendMouseWheelScroll(ScrollAxis.VERTICAL, -e.deltaY);
      }
    }
  }

  // Block browser context menu so as not to obscure the context menu
  // on the remote machine.
  function handleContextMenu(e: React.MouseEvent) {
    e.preventDefault();
  }

  function handleCtrlAltDel() {
    if (!client) {
      return;
    }
    client.sendKeyboardInput('ControlLeft', ButtonState.DOWN);
    client.sendKeyboardInput('AltLeft', ButtonState.DOWN);
    client.sendKeyboardInput('Delete', ButtonState.DOWN);
  }

  return (
    <Flex
      flexDirection="column"
      css={`
        // Fill the window.
        position: absolute;
        width: 100%;
        height: 100%;
      `}
    >
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
          client.shutdown();
        }}
        userHost={`${username}@${hostname}`}
        canShareDirectory={directorySharingPossible(directorySharingState)}
        isSharingDirectory={isSharingDirectory(directorySharingState)}
        isSharingClipboard={isSharingClipboard(clipboardSharingState)}
        clipboardSharingMessage={clipboardSharingMessage(clipboardSharingState)}
        onShareDirectory={onShareDirectory}
        onCtrlAltDel={handleCtrlAltDel}
        alerts={alerts}
        onRemoveAlert={onRemoveAlert}
      />

      {screenState.screen === 'anotherSessionActive' && (
        <AnotherSessionActiveDialog {...props} />
      )}
      {screenState.screen === 'mfa' && <AuthnDialog mfaState={mfa} />}
      {screenState.screen === 'alert dialog' && (
        <AlertDialog screenState={screenState} />
      )}
      {screenState.screen === 'processing' && <Processing />}

      <TdpClientCanvas
        ref={tdpClientCanvasRef}
        style={{
          display: screenState.canvasState.shouldDisplay ? 'flex' : 'none',
        }}
        onKeyDown={handleKeyDown}
        onKeyUp={handleKeyUp}
        onBlur={handleBlur}
        onMouseMove={handleMouseMove}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        onMouseWheel={handleMouseWheel}
        onContextMenu={handleContextMenu}
        onResize={client?.resize}
      />
    </Flex>
  );
}

const AlertDialog = ({ screenState }: { screenState: ScreenState }) => (
  <Dialog dialogCss={() => ({ width: '484px' })} open={true}>
    <DialogHeader style={{ flexDirection: 'column' }}>
      <DialogTitle>Disconnected</DialogTitle>
    </DialogHeader>
    <DialogContent>
      <>
        {typeof screenState.alertMessage === 'object' ? (
          <Info details={screenState.alertMessage.message}>
            {screenState.alertMessage.title}
          </Info>
        ) : (
          <Info>{screenState.alertMessage}</Info>
        )}
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
);

const AnotherSessionActiveDialog = (props: State) => {
  return (
    <Dialog
      dialogCss={() => ({ width: '484px' })}
      onClose={() => {}}
      open={true}
    >
      <DialogHeader style={{ flexDirection: 'column' }}>
        <DialogTitle>Another Session Is Active</DialogTitle>
      </DialogHeader>
      <DialogContent>
        This desktop has an active session, connecting to it may close the other
        session. Do you wish to continue?
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
            props.setShowAnotherSessionActiveDialog(false);
          }}
        >
          Continue
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};

const Processing = () => {
  return (
    <Box textAlign="center" m={10}>
      <Indicator />
    </Box>
  );
};

const invalidStateMessage = 'internal application error';

/**
 * Calculate the next `ScreenState` based on the current state and the latest
 * attempts to fetch the desktop session, connect to the TDP server, and connect
 * to the websocket.
 */
const nextScreenState = (
  prevState: ScreenState,
  fetchAttempt: Attempt,
  tdpConnection: Attempt,
  wsConnection: WebsocketAttempt,
  showAnotherSessionActiveDialog: boolean,
  mfa: MfaState
): ScreenState => {
  // We always want to show the user the first alert that caused the session to fail/end,
  // so if we're already showing an alert, don't change the screen.
  //
  // This allows us to track the various pieces of the state independently and always display
  // the vital information to the user. For example, we can track the TDP connection status
  // and the websocket connection status separately throughout the codebase. If the TDP connection
  // fails, and then the websocket closes, we want to show the `tdpConnection.statusText` to the user,
  // not the `wsConnection.statusText`. But if the websocket closes unexpectedly before a TDP message telling
  // us why, we want to show the websocket closing message to the user.
  if (prevState.screen === 'alert dialog') {
    return prevState;
  }

  // Otherwise, calculate a new screen state.
  const showAnotherSessionActive = showAnotherSessionActiveDialog;
  const showMfa = shouldShowMfaPrompt(mfa);
  const showAlert =
    fetchAttempt.status === 'failed' || // Fetch attempt failed
    tdpConnection.status === 'failed' || // TDP connection closed by the remote side.
    mfa.attempt.status === 'error' || // MFA was canceled
    wsConnection.status === 'closed'; // Websocket closed, means unexpected close.

  const atLeastOneAttemptProcessing =
    fetchAttempt.status === 'processing' ||
    tdpConnection.status === 'processing';
  const noDialogs = !(showMfa || showAnotherSessionActive || showAlert);
  const showProcessing = atLeastOneAttemptProcessing && noDialogs;

  if (showAnotherSessionActive) {
    // Highest priority: we don't want to connect (`shouldConnect`) until
    // the user has decided whether to continue with the active session.
    return {
      screen: 'anotherSessionActive',
      canvasState: { shouldConnect: false, shouldDisplay: false },
    };
  } else if (showMfa) {
    // Second highest priority. Secondary to `showAnotherSessionActive` because
    // this won't happen until the user has decided whether to continue with the active session.
    //
    // `shouldConnect` is true because we want to maintain the websocket connection that the mfa
    // request was made over.
    return {
      screen: 'mfa',
      canvasState: { shouldConnect: true, shouldDisplay: false },
    };
  } else if (showAlert) {
    // Third highest priority. If either attempt or the websocket has failed, show the alert.
    return {
      screen: 'alert dialog',
      alertMessage: calculateAlertMessage(
        fetchAttempt,
        tdpConnection,
        wsConnection,
        showAnotherSessionActiveDialog,
        mfa.attempt,
        prevState
      ),
      canvasState: { shouldConnect: false, shouldDisplay: false },
    };
  } else if (showProcessing) {
    // Fourth highest priority. If at least one attempt is still processing, show the processing indicator
    // while trying to connect to the TDP server via the websocket.
    const shouldConnect = fetchAttempt.status !== 'processing';
    return {
      screen: 'processing',
      canvasState: { shouldConnect, shouldDisplay: false },
    };
  } else {
    // Default state: everything is good, so show the canvas.
    return {
      screen: 'canvas',
      canvasState: { shouldConnect: true, shouldDisplay: true },
    };
  }
};

/**
 * Calculate the error message to display to the user based on the current state.
 */
/* eslint-disable no-console */
const calculateAlertMessage = (
  fetchAttempt: Attempt,
  tdpConnection: Attempt,
  wsConnection: WebsocketAttempt,
  showAnotherSessionActiveDialog: boolean,
  mfaAttempt: AsyncAttempt<unknown>,
  prevState: ScreenState
) => {
  let message = '';
  // Errors, except for dialog cancellations, are handled within the MFA dialog.
  if (mfaAttempt.status === 'error') {
    return {
      title: 'This session requires multi factor authentication',
      message: mfaAttempt.statusText,
    };
  }
  if (fetchAttempt.status === 'failed') {
    message = fetchAttempt.statusText || 'fetch attempt failed';
  } else if (tdpConnection.status === 'failed') {
    message = tdpConnection.statusText || 'Disconnected';
  } else if (wsConnection.status === 'closed') {
    message =
      wsConnection.statusText || 'websocket disconnected for an unknown reason';
  } else {
    console.error('invalid state');
    console.error({
      fetchAttempt,
      tdpConnection,
      wsConnection,
      showAnotherSessionActiveDialog,
      prevState,
    });
    message = invalidStateMessage;
  }
  return message;
};
/* eslint-enable no-console */

type ScreenState = {
  screen:
    | 'mfa'
    | 'anotherSessionActive'
    | 'alert dialog'
    | 'processing'
    | 'canvas';

  alertMessage?: string | { title: string; message: string };
  canvasState: {
    shouldConnect: boolean;
    shouldDisplay: boolean;
  };
};
