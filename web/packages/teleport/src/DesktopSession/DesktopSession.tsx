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
    onClipboardData,
    sendLocalClipboardToRemote,
    clientScreenSpecToRequest,
    clipboardSharingState,
    setClipboardSharingState,
    onShareDirectory,
    alerts,
    onRemoveAlert,
    fetchAttempt,
    showAnotherSessionActiveDialog,
    addAlert,
  } = props;
  const [tdpConnectionStatus, setTdpConnectionStatus] =
    useState<TdpConnectionStatus>({ status: '' });

  const keyboardHandler = useRef(new KeyboardHandler());
  useEffect(() => {
    return () => keyboardHandler.current.dispose();
  }, []);

  const tdpClientCanvasRef = useRef<TdpClientCanvasRef>(null);
  const initialTdpConnectionSucceeded = useRef(false);
  const onInitialTdpConnectionSucceeded = useCallback(() => {
    // The first image fragment we see signals a successful TDP connection.
    if (initialTdpConnectionSucceeded.current) {
      return;
    }
    initialTdpConnectionSucceeded.current = true;
    setTdpConnectionStatus({ status: 'active' });

    // Focus the canvas once the canvas is visible.
    // It needs to happen in the next tick, otherwise id doesn't work.
    setTimeout(() => tdpClientCanvasRef.current?.focus());
  }, []);

  useListener(client?.onClipboardData, onClipboardData);

  const handleFatalError = useCallback(
    (error: Error) => {
      setDirectorySharingState(defaultDirectorySharingState);
      setClipboardSharingState(defaultClipboardSharingState);
      setTdpConnectionStatus({
        status: 'disconnected',
        message: error.message || error.toString(),
      });
      initialTdpConnectionSucceeded.current = false;
    },
    [setClipboardSharingState, setDirectorySharingState]
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
        setTdpConnectionStatus({ status: 'disconnected', message: statusText });
        initialTdpConnectionSucceeded.current = false;
      },
      [setTdpConnectionStatus]
    )
  );
  useListener(
    client?.onWsOpen,
    useCallback(() => {
      setTdpConnectionStatus({ status: 'connected' });
    }, [setTdpConnectionStatus])
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

  const shouldConnect =
    fetchAttempt.status === 'success' && !showAnotherSessionActiveDialog;
  useEffect(() => {
    if (!(client && shouldConnect)) {
      return;
    }
    void client.connect(tdpClientCanvasRef.current.getSize());
    return () => {
      client.shutdown();
    };
  }, [client, shouldConnect]);

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

  const screenState = getScreenState(
    fetchAttempt,
    tdpConnectionStatus,
    showAnotherSessionActiveDialog,
    mfa
  );

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

      {screenState.state === 'another-session-active' && (
        <AnotherSessionActiveDialog
          onContinue={() => props.setShowAnotherSessionActiveDialog(false)}
          onAbort={() => window.close()}
        />
      )}
      {screenState.state === 'mfa' && <AuthnDialog mfaState={mfa} />}
      {screenState.state === 'error' && (
        <AlertDialog
          message={screenState.message}
          onRetry={() => window.location.reload()}
        />
      )}
      {screenState.state === 'processing' && <Processing />}

      <TdpClientCanvas
        ref={tdpClientCanvasRef}
        hidden={screenState.state !== 'canvas-visible'}
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

const AlertDialog = (props: {
  message: { title: string; details?: string };
  onRetry(): void;
}) => (
  <Dialog dialogCss={() => ({ width: '484px' })} open={true}>
    <DialogHeader style={{ flexDirection: 'column' }}>
      <DialogTitle>Disconnected</DialogTitle>
    </DialogHeader>
    <DialogContent>
      <Info details={props.message.details}>{props.message.title}</Info>
      Refresh the page to reconnect.
    </DialogContent>
    <DialogFooter>
      <ButtonSecondary size="large" width="30%" onClick={props.onRetry}>
        Refresh
      </ButtonSecondary>
    </DialogFooter>
  </Dialog>
);

const AnotherSessionActiveDialog = (props: {
  onAbort(): void;
  onContinue(): void;
}) => {
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
        <ButtonPrimary mr={3} onClick={props.onAbort}>
          Abort
        </ButtonPrimary>
        <ButtonSecondary onClick={props.onContinue}>Continue</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};

const Processing = () => {
  return (
    <Box
      // Position the indicator in the center of the screen without taking space.
      css={`
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
      `}
    >
      <Indicator delay="none" />
    </Box>
  );
};

function getScreenState(
  fetchAttempt: Attempt,
  tdpConnectionStatus: TdpConnectionStatus,
  showAnotherSessionActiveDialog: boolean,
  mfa: MfaState
): ScreenState {
  if (fetchAttempt.status === 'failed') {
    return {
      state: 'error',
      message: {
        title: 'Could not fetch session details',
        details: fetchAttempt.statusText,
      },
    };
  }
  if (tdpConnectionStatus.status === 'disconnected') {
    return {
      state: 'error',
      message: { title: tdpConnectionStatus.message },
    };
  }
  // Errors, except for dialog cancellations, are handled within the MFA dialog.
  if (mfa.attempt.status === 'error' && !shouldShowMfaPrompt(mfa)) {
    return {
      state: 'error',
      message: {
        title: 'This session requires multi factor authentication',
        details: mfa.attempt.statusText,
      },
    };
  }

  if (showAnotherSessionActiveDialog) {
    return { state: 'another-session-active' };
  }

  if (shouldShowMfaPrompt(mfa)) {
    return { state: 'mfa' };
  }

  if (tdpConnectionStatus.status === 'active') {
    return { state: 'canvas-visible' };
  }

  return { state: 'processing' };
}

/** Describes state of the TDP connection. */
type TdpConnectionStatus =
  /** Unknown status. It may be idle or in the process of connecting. */
  | { status: '' }
  /** The transport layer connection has been successfully established. */
  | { status: 'connected' }
  /** The remote desktop is visible, we received the first frame. */
  | { status: 'active' }
  /**
   * The client has disconnected.
   * This can happen either gracefully (on the remote side)
   * or due to closing the connection.
   */
  | {
      status: 'disconnected';
      message: string;
    };

type ScreenState =
  | { state: 'another-session-active' }
  | { state: 'mfa' }
  | { state: 'processing' }
  | { state: 'canvas-visible' }
  | {
      state: 'error';
      message: { title: string; details?: string };
    };
