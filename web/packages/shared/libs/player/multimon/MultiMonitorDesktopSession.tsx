/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

/**
 * Multi-monitor RDP session on the classic canvas (fast-path bitmap) rendering
 * path — the "one client, N canvases, routed" boundary.
 *
 * A single {@link TdpClient} streams one framebuffer spanning the whole monitor
 * arrangement; {@link TdpMonitorSession} routes each fast-path `BitmapFrame` to
 * the right monitor's `CanvasRenderer`. The main monitor renders inline in this
 * window; popup monitors render into `window.open` child windows via
 * `createPortal` (their `<canvas>` uses inline styles, so no stylesheet needs
 * to cross the window boundary).
 *
 * This is the deliberately-rough integration described in
 * {@link TdpMonitorSession}: it wires the isolated multi-monitor UI to the live
 * canvas path so the flow is exercisable. Known rough edges are marked ROUGH.
 */

import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import styled from 'styled-components';

import {
  CanvasRenderer,
  type CanvasRendererRef,
} from 'shared/components/CanvasRenderer';
import { InputHandler } from 'shared/components/DesktopSession/InputHandler';
import {
  ButtonState,
  type MouseButton,
  ScrollAxis,
  type TdpClient,
} from 'shared/libs/tdp';

import type { ModelMonitor } from '../monitors/monitorModel';
import type { ManagedMonitorView } from '../monitors/monitorSession';
import { MonitorManagerPanel } from '../monitors/MonitorManagerPanel';
import { MonitorTaskbar } from '../monitors/MonitorTaskbar';
import { useScreenTopology } from '../monitors/useScreenTopology';
import { TdpMonitorSession } from './TdpMonitorSession';

export interface MultiMonitorDesktopSessionProps {
  /** A live, not-yet-connected TdpClient for this desktop. */
  client: TdpClient;
  /** Seed monitors (usually just the main window). */
  initial: ModelMonitor[];
  maxMonitors: number;
  /** RDP scale percent (e.g. device DPR * 100). */
  scale: number;
  /** Builds the URL loaded into a popup monitor window. */
  popupUrlFactory?: (id: number) => string;
  statusLabel?: string;
}

export function MultiMonitorDesktopSession({
  client,
  initial,
  maxMonitors,
  scale,
  popupUrlFactory,
  statusLabel,
}: MultiMonitorDesktopSessionProps) {
  const session = useMemo(
    () => new TdpMonitorSession(client, { initial, maxMonitors, scale }),
    // Intentionally constructed once per client.
    [client] // eslint-disable-line react-hooks/exhaustive-deps
  );
  const [state, setState] = useState(() => session.getState());
  useEffect(() => session.subscribe(setState), [session]);

  // The component owns the session lifecycle, including connecting the client
  // with the seed layout's bounding box as the initial screen spec. Subsequent
  // layout changes push their own screen specs from the session.
  useEffect(() => {
    client.connect({ screenSpec: session.getInitialScreenSpec() }).catch(() => {
      // ROUGH: connection errors should surface to the taskbar/status UI.
    });
  }, [client, session]);

  const topology = useScreenTopology();
  const [panelOpen, setPanelOpen] = useState(false);

  // One shared InputHandler drives every monitor's canvas: modifier-sync and
  // key-withholding are session-global, not per-window.
  const inputHandler = useRef(new InputHandler());
  useEffect(() => () => inputHandler.current.dispose(), []);

  const mainView = state.monitors.find(m => m.role === 'main');
  const popupViews = state.monitors.filter(m => m.role === 'popup');

  return (
    <Root>
      {mainView && (
        <MonitorCanvas
          view={mainView}
          session={session}
          client={client}
          input={inputHandler.current}
        />
      )}

      {popupViews.map(view => (
        <PopupMonitorWindow
          key={view.id}
          view={view}
          session={session}
          client={client}
          input={inputHandler.current}
          url={popupUrlFactory?.(view.id) ?? 'about:blank'}
        />
      ))}

      <MonitorTaskbar
        monitorCount={state.monitors.length}
        maxMonitors={state.maxMonitors}
        statusLabel={statusLabel}
        restorable={state.restorable}
        onOpenMonitors={() => setPanelOpen(true)}
        onRestore={() => session.restoreMonitors()}
      />

      <MonitorManagerPanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        session={session}
        state={state}
        topology={topology}
      />
    </Root>
  );
}

/**
 * Renders one monitor's `CanvasRenderer`, registers it with the session, sizes
 * its backing buffer to the monitor's physical viewport, and forwards input —
 * offsetting local coordinates into whole-desktop space.
 */
function MonitorCanvas({
  view,
  session,
  client,
  input,
}: {
  view: ManagedMonitorView;
  session: TdpMonitorSession;
  client: TdpClient;
  input: InputHandler;
}) {
  const canvasRef = useRef<CanvasRendererRef>(null);

  // Register / size on mount and whenever this monitor's viewport changes.
  useEffect(() => {
    const ref = canvasRef.current;
    if (!ref) return;
    session.registerCanvas(view.id, ref);
    return () => session.unregisterCanvas(view.id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, view.id]);

  useEffect(() => {
    const vp = session.getViewport(view.id);
    if (vp) canvasRef.current?.setResolution({ width: vp.width, height: vp.height });
  }, [session, view.id, view.rect.width, view.rect.height]);

  // Map a canvas-local mouse event to whole-desktop framebuffer coords.
  function toDesktopCoords(e: React.MouseEvent) {
    const canvas = e.currentTarget as HTMLCanvasElement;
    const rect = canvas.getBoundingClientRect();
    const s = Math.min(1, rect.width / canvas.width, rect.height / canvas.height);
    const offsetX = (rect.width - canvas.width * s) / 2;
    const offsetY = (rect.height - canvas.height * s) / 2;
    const localX = Math.round((e.clientX - rect.left - offsetX) / s);
    const localY = Math.round((e.clientY - rect.top - offsetY) / s);
    const inBounds =
      localX >= 0 && localY >= 0 && localX < canvas.width && localY < canvas.height;
    const vp = session.getViewport(view.id);
    return {
      x: localX + (vp?.x ?? 0),
      y: localY + (vp?.y ?? 0),
      inBounds,
    };
  }

  return (
    <CanvasRenderer
      ref={canvasRef}
      onKeyDown={e =>
        input.handleInputEvent({
          cli: client,
          e: e.nativeEvent,
          state: ButtonState.DOWN,
        })
      }
      onKeyUp={e =>
        input.handleInputEvent({
          cli: client,
          e: e.nativeEvent,
          state: ButtonState.UP,
        })
      }
      onBlur={() => input.onFocusOut()}
      onMouseMove={e => {
        const p = toDesktopCoords(e);
        if (p.inBounds) client.sendMouseMove(p.x, p.y);
      }}
      onMouseDown={e => {
        const p = toDesktopCoords(e);
        if (!p.inBounds) return;
        // Position the server cursor at the press location first, so the click
        // lands on the right monitor's slice, then send the button.
        client.sendMouseMove(p.x, p.y);
        input.handleInputEvent({
          cli: client,
          e: e.nativeEvent,
          state: ButtonState.DOWN,
        });
      }}
      onMouseUp={e =>
        input.handleInputEvent({
          cli: client,
          e: e.nativeEvent,
          state: ButtonState.UP,
        })
      }
      onMouseWheel={e => {
        e.preventDefault();
        input.synchronizeModifierState(client, e);
        if (e.deltaMode === WheelEvent.DOM_DELTA_PIXEL) {
          if (e.deltaX) client.sendMouseWheelScroll(ScrollAxis.HORIZONTAL, -e.deltaX);
          if (e.deltaY) client.sendMouseWheelScroll(ScrollAxis.VERTICAL, -e.deltaY);
        }
      }}
      onContextMenu={e => e.preventDefault()}
    />
  );
}

/**
 * ROUGH: opens (and owns) an OS popup window for a popup monitor and portals a
 * `MonitorCanvas` into it. Real multi-monitor needs the popup to survive
 * reload and to report its true content position back into the layout (the
 * codec branch handles both); here we cover the essential open → render →
 * register → close-detection loop.
 */
function PopupMonitorWindow({
  view,
  session,
  client,
  input,
  url,
}: {
  view: ManagedMonitorView;
  session: TdpMonitorSession;
  client: TdpClient;
  input: InputHandler;
  url: string;
}) {
  const [win, setWin] = useState<Window | null>(null);
  const [container, setContainer] = useState<HTMLElement | null>(null);

  useEffect(() => {
    const features = view.physical
      ? `popup,left=${view.physical.left},top=${view.physical.top},width=${view.cssWidth},height=${view.cssHeight}`
      : `popup,width=${view.cssWidth},height=${view.cssHeight}`;
    const w = window.open(url, `monitor-${view.id}`, features);
    if (!w) {
      // Popup blocked — surface as a blocked monitor and bail. ROUGH: no retry.
      return;
    }
    setWin(w);

    const mount = () => {
      const el = w.document.createElement('div');
      el.style.cssText = 'position:fixed;inset:0;margin:0;';
      w.document.body.style.margin = '0';
      w.document.body.appendChild(el);
      setContainer(el);
    };
    if (w.document.readyState === 'complete') mount();
    else w.addEventListener('load', mount, { once: true });

    // Detect the user closing the popup and drop the monitor from the layout.
    const poll = window.setInterval(() => {
      if (w.closed) {
        window.clearInterval(poll);
        session.removeMonitor(view.id);
      }
    }, 500);

    return () => {
      window.clearInterval(poll);
      if (!w.closed) w.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view.id]);

  if (!win || !container) return null;
  return createPortal(
    <MonitorCanvas view={view} session={session} client={client} input={input} />,
    container
  );
}

const Root = styled.div`
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: ${p => p.theme.colors.levels.sunken};
`;
