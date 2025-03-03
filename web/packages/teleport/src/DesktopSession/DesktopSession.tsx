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

import { Box, ButtonPrimary, ButtonSecondary, Flex, Indicator } from 'design';
import { Info } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Attempt } from 'shared/hooks/useAttemptNext';

import AuthnDialog from 'teleport/components/AuthnDialog';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import { TdpClientCanvasRef } from 'teleport/components/TdpClientCanvas/TdpClientCanvas';
import { useListener } from 'teleport/lib/tdp/client';
import type { MfaState } from 'teleport/lib/useMfa';

import TopBar from './TopBar';
import useDesktopSession, {
  clipboardSharingMessage,
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
    clientOnClipboardData,
    clientOnTdpError,
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
    onResize,
    clientScreenSpecToRequest,
    clipboardSharingState,
    setClipboardSharingState,
    onShareDirectory,
    onCtrlAltDel,
    alerts,
    onRemoveAlert,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    showAnotherSessionActiveDialog,
    addAlert,
  } = props;

  const [screenState, setScreenState] = useState<ScreenState>({
    screen: 'processing',
    canvasState: { shouldConnect: false, shouldDisplay: false },
  });

  useListener(client?.onClipboardData, clientOnClipboardData);
  useListener(client?.onError, clientOnTdpError);
  useListener(client?.onClientError, clientOnTdpError);
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
  useListener(client?.onInfo, clientOnTdpInfo);
  useListener(client?.onWsClose, clientOnWsClose);
  useListener(client?.onWsOpen, clientOnWsOpen);

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
        onCtrlAltDel={onCtrlAltDel}
        alerts={alerts}
        onRemoveAlert={onRemoveAlert}
      />

      {screenState.screen === 'anotherSessionActive' && (
        <AnotherSessionActiveDialog {...props} />
      )}
      {screenState.screen === 'mfa' && <MfaDialog mfa={mfa} />}
      {screenState.screen === 'alert dialog' && (
        <AlertDialog screenState={screenState} />
      )}
      {screenState.screen === 'processing' && <Processing />}

      <TdpClientCanvas
        ref={tdpClientCanvasRef}
        style={{
          display: screenState.canvasState.shouldDisplay ? 'flex' : 'none',
        }}
        onKeyDown={canvasOnKeyDown}
        onKeyUp={canvasOnKeyUp}
        onBlur={canvasOnFocusOut}
        onMouseMove={canvasOnMouseMove}
        onMouseDown={canvasOnMouseDown}
        onMouseUp={canvasOnMouseUp}
        onMouseWheel={canvasOnMouseWheelScroll}
        onContextMenu={canvasOnContextMenu}
        onResize={onResize}
      />
    </Flex>
  );
}

const MfaDialog = ({ mfa }: { mfa: MfaState }) => {
  return (
    <AuthnDialog
      mfaState={mfa}
      replaceErrorText={
        'This session requires multi factor authentication to continue. Please hit try again and follow the prompts given by your browser to complete authentication.'
      }
    />
  );
};

const AlertDialog = ({ screenState }: { screenState: ScreenState }) => (
  <Dialog dialogCss={() => ({ width: '484px' })} open={true}>
    <DialogHeader style={{ flexDirection: 'column' }}>
      <DialogTitle>Disconnected</DialogTitle>
    </DialogHeader>
    <DialogContent>
      <>
        <Info
          children={<>{screenState.alertMessage || invalidStateMessage}</>}
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
  webauthn: MfaState
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
  const showMfa = webauthn.challenge;
  const showAlert =
    fetchAttempt.status === 'failed' || // Fetch attempt failed
    tdpConnection.status === 'failed' || // TDP connection closed by the remote side.
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
  prevState: ScreenState
): string => {
  let message = '';
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

  alertMessage?: string;
  canvasState: {
    shouldConnect: boolean;
    shouldDisplay: boolean;
  };
};
