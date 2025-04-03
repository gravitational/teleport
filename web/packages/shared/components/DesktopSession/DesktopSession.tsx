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
import {
  CanvasRenderer,
  CanvasRendererRef,
} from 'shared/components/CanvasRenderer';
import { Attempt, makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';
import {
  ButtonState,
  ScrollAxis,
  TdpClient,
  useListener,
} from 'shared/libs/tdp';

import { KeyboardHandler } from './KeyboardHandler';
import TopBar from './TopBar';
import useDesktopSession, {
  clipboardSharingMessage,
  defaultClipboardSharingState,
  defaultDirectorySharingState,
  directorySharingPossible,
  isSharingClipboard,
  isSharingDirectory,
} from './useDesktopSession';

export interface DesktopSessionProps {
  client: TdpClient;
  username: string;
  desktop: string;
  aclAttempt: Attempt<{
    clipboardSharingEnabled: boolean;
    directorySharingEnabled: boolean;
  }>;
  /**
   * Injects a custom component that overrides other connection states.
   * Useful for per-session MFA, which differs between Web UI and Connect.
   * Provides a callback to retry the connection.
   */
  customConnectionState?(args: { retry(): void }): React.ReactElement;
  hasAnotherSession(): Promise<boolean>;
}

export function DesktopSession({
  client,
  aclAttempt,
  username,
  desktop,
  hasAnotherSession,
  customConnectionState,
}: DesktopSessionProps) {
  const {
    directorySharingState,
    setDirectorySharingState,
    onClipboardData,
    sendLocalClipboardToRemote,
    clipboardSharingState,
    setClipboardSharingState,
    onShareDirectory,
    alerts,
    onRemoveAlert,
    addAlert,
  } = useDesktopSession(client, aclAttempt);

  const [tdpConnectionStatus, setTdpConnectionStatus] =
    useState<TdpConnectionStatus>({ status: '' });

  const keyboardHandler = useRef(new KeyboardHandler());
  useEffect(() => {
    return () => keyboardHandler.current.dispose();
  }, []);

  const [
    anotherDesktopActiveAttempt,
    runCheckIsAnotherDesktopActive,
    setAnotherDesktopActiveAttempt,
  ] = useAsync(hasAnotherSession);

  useEffect(() => {
    if (anotherDesktopActiveAttempt.status === '') {
      runCheckIsAnotherDesktopActive();
    }
  }, [anotherDesktopActiveAttempt.status, runCheckIsAnotherDesktopActive]);

  const canvasRendererRef = useRef<CanvasRendererRef>(null);
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
    setTimeout(() => canvasRendererRef.current?.focus());
  }, []);

  useListener(client.onClipboardData, onClipboardData);

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
  useListener(client.onError, handleFatalError);

  const addWarning = useCallback(
    (warning: string) => {
      addAlert({
        content: warning,
        severity: 'warn',
      });
    },
    [addAlert]
  );
  useListener(client.onWarning, addWarning);
  useListener(client.onClientWarning, addWarning);

  useListener(
    client.onInfo,
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
    client.onTransportClose,
    useCallback(
      statusText => {
        setTdpConnectionStatus({ status: 'disconnected', message: statusText });
        initialTdpConnectionSucceeded.current = false;
      },
      [setTdpConnectionStatus]
    )
  );
  useListener(
    client.onTransportOpen,
    useCallback(() => {
      setTdpConnectionStatus({ status: 'connected' });
    }, [setTdpConnectionStatus])
  );

  useListener(client.onPointer, canvasRendererRef.current?.setPointer);
  useListener(
    client.onPngFrame,
    useCallback(
      frame => {
        onInitialTdpConnectionSucceeded();
        canvasRendererRef.current?.renderPngFrame(frame);
      },
      [onInitialTdpConnectionSucceeded]
    )
  );
  useListener(
    client.onBmpFrame,
    useCallback(
      frame => {
        onInitialTdpConnectionSucceeded();
        canvasRendererRef.current?.renderBitmapFrame(frame);
      },
      [onInitialTdpConnectionSucceeded]
    )
  );
  useListener(client.onReset, canvasRendererRef.current?.clear);
  useListener(client.onScreenSpec, canvasRendererRef.current?.setResolution);

  const shouldConnect =
    aclAttempt.status === 'success' &&
    anotherDesktopActiveAttempt.status === 'success' &&
    !anotherDesktopActiveAttempt.data;
  useEffect(() => {
    if (!shouldConnect) {
      return;
    }
    void client.connect(canvasRendererRef.current.getSize());
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
    client.sendKeyboardInput('ControlLeft', ButtonState.DOWN);
    client.sendKeyboardInput('AltLeft', ButtonState.DOWN);
    client.sendKeyboardInput('Delete', ButtonState.DOWN);
  }

  //TODO(gzdunek): Replace with client.connect(), so that we don't refresh the entire page.
  const onRetry = () => window.location.reload();
  const screenState = getScreenState(
    aclAttempt,
    anotherDesktopActiveAttempt,
    tdpConnectionStatus,
    customConnectionState?.({ retry: onRetry })
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
        userHost={`${username} on ${desktop}`}
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
          onContinue={() =>
            setAnotherDesktopActiveAttempt(makeSuccessAttempt(false))
          }
          onAbort={() => window.close()}
        />
      )}
      {screenState.state === 'custom' && screenState.component}
      {screenState.state === 'error' && (
        <AlertDialog message={screenState.message} onRetry={onRetry} />
      )}
      {screenState.state === 'processing' && <Processing />}

      <CanvasRenderer
        ref={canvasRendererRef}
        hidden={screenState.state !== 'canvas-visible'}
        onKeyDown={handleKeyDown}
        onKeyUp={handleKeyUp}
        onBlur={handleBlur}
        onMouseMove={handleMouseMove}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        onMouseWheel={handleMouseWheel}
        onContextMenu={handleContextMenu}
        onResize={client.resize}
      />
    </Flex>
  );
}

export const AlertDialog = (props: {
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
  aclAttempt: Attempt<unknown>,
  anotherDesktopActiveAttempt: Attempt<unknown>,
  tdpConnectionStatus: TdpConnectionStatus,
  customConnectionState: React.ReactElement | undefined
): ScreenState {
  if (customConnectionState) {
    return { state: 'custom', component: customConnectionState };
  }

  if (aclAttempt.status === 'error') {
    return {
      state: 'error',
      message: {
        title: 'Could not fetch session details',
        details: aclAttempt.statusText,
      },
    };
  }
  if (anotherDesktopActiveAttempt.status === 'error') {
    return {
      state: 'error',
      message: {
        title: 'Could not fetch session details',
        details: anotherDesktopActiveAttempt.statusText,
      },
    };
  }
  if (tdpConnectionStatus.status === 'disconnected') {
    return {
      state: 'error',
      message: { title: tdpConnectionStatus.message },
    };
  }

  if (
    anotherDesktopActiveAttempt.status === 'success' &&
    anotherDesktopActiveAttempt.data
  ) {
    return { state: 'another-session-active' };
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
  | { state: 'custom'; component: React.JSX.Element }
  | { state: 'another-session-active' }
  | { state: 'processing' }
  | { state: 'canvas-visible' }
  | {
      state: 'error';
      message: { title: string; details?: string };
    };
