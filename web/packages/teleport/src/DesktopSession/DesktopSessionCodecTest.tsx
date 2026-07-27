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

import { useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router';

import { DesktopSessionPopupDisplay } from 'shared/libs/player/DesktopSessionPopupDisplay';
import type { MonitorSpec } from 'shared/libs/player/DesktopSessionTest';
import { DesktopSessionTestMulti } from 'shared/libs/player/DesktopSessionTestMulti';
import {
  loadLayout,
  type SavedLayout,
} from 'shared/libs/player/monitors/monitorPersistence';

import cfg, { type UrlDesktopParams } from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';

// System-wide cap mirrored from server-side (lib/srv/desktop/rdp/rdpclient/client.go).
const MAX_MONITORS = 4;

/**
 * Test harness route for exercising the new Rust `codec` crate end-to-end
 * against a live desktop. Same URL params as `DesktopSession` plus a
 * `/codec-test` suffix.
 *
 * It launches straight into the session as a single monitor (the main window).
 * Additional monitors are spawned on demand from the in-session "Monitors"
 * taskbar, and the arrangement is remembered across sessions: on reconnect the
 * taskbar offers a one-click "Restore N monitors" to reopen the previous
 * popups at their saved positions. (Popups need a user gesture to open, hence
 * the button rather than auto-open.)
 */

/// Map the browser's `devicePixelRatio` to an RDP-style scale percentage
/// (e.g. DPR 2 → 200, DPR 1.5 → 150). The codec-test harness hard-codes
/// this onto the ClientScreenSpec so the Windows server renders at the
/// physical pixel density of the user's display; we deliberately don't
/// expose a user-facing scale picker on this page.
function dprPercent(): number {
  return Math.max(100, Math.round((window.devicePixelRatio ?? 1) * 100));
}

export function DesktopSessionCodecTest() {
  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();
  const location = useLocation();
  const popupConfig = useMemo(
    () => parsePopupConfig(location.search),
    [location.search]
  );

  // Per-desktop key for the remembered monitor arrangement.
  const layoutStorageKey = `codec-test:monitors:${clusterId}:${desktopName}`;
  // Capture the previous session's layout once, synchronously, BEFORE the
  // session component starts persisting/clearing — so a session that never
  // restores can forget it without losing the restore offer.
  const [restoreLayout] = useState<SavedLayout | null>(() =>
    popupConfig ? null : loadLayout(layoutStorageKey)
  );

  useEffect(() => {
    document.title = popupConfig
      ? `codec-test popup#${popupConfig.monitorIndex} • ${desktopName}`
      : `codec-test • ${username} on ${desktopName} • ${clusterId}`;
  }, [clusterId, desktopName, username, popupConfig]);

  const wsUrl = cfg.api.desktopWsAddr
    .replace(':fqdn', getHostName())
    .replace(':clusterId', clusterId)
    .replace(':desktopName', desktopName)
    .replace(':username', username)
    .replace(':version', 'teleport-tdpb-1.0');

  // Popup-mode short-circuit: render the passive display component. It has
  // no WebSocket/decoder of its own — pixels arrive from the main window via
  // `window.opener.postMessage`, input events flow back the same way.
  if (popupConfig) {
    return (
      <DesktopSessionPopupDisplay
        monitors={popupConfig.monitors}
        monitorIndex={popupConfig.monitorIndex}
      />
    );
  }

  // Main window: connect immediately as a single monitor sized to this window.
  // The shared SharedWorker-hosted decoder drives one wasm framebuffer + N
  // registered canvases regardless of count, so adding/restoring monitors
  // mid-session is identical to starting with one.
  const singleMonitor: MonitorSpec = {
    x: 0,
    y: 0,
    width: window.innerWidth,
    height: window.innerHeight,
    isPrimary: true,
  };
  return (
    <DesktopSessionTestMulti
      wsUrl={wsUrl}
      bearerToken={getAccessToken()}
      username={username}
      monitors={[singleMonitor]}
      monitorIndex={0}
      mode="main"
      // Hard-coded to the current display's DPR: Windows will render at the
      // screen's actual pixel density (e.g. 200 for Retina), and our canvas is
      // sized in CSS pixels so it remains crisp.
      scale={dprPercent()}
      // Lets the in-session "Monitors" taskbar add displays mid-session.
      popupUrlFactory={makePopupUrl}
      maxMonitors={MAX_MONITORS}
      // Remember the arrangement + offer one-click restore on reconnect.
      layoutStorageKey={layoutStorageKey}
      restoreLayout={restoreLayout}
    />
  );
}

function buildPopupQuery(monitors: MonitorSpec[], monitorIndex: number): string {
  const params = new URLSearchParams();
  params.set('popup', '1');
  params.set('monitorIndex', String(monitorIndex));
  params.set('monitors', btoa(JSON.stringify(monitors)));
  return params.toString();
}

/**
 * Builds a popup URL for a monitor added mid-session via the management UI. The
 * passive popup display only needs `popup=1`, its own `monitorIndex`, and a
 * `monitors` array long enough to validate — it waits for the `init` message
 * from main for its real viewport, so the contents here are placeholders.
 */
function makePopupUrl(monitorIndex: number): string {
  const placeholder: MonitorSpec[] = Array.from(
    { length: monitorIndex + 1 },
    () => ({ x: 0, y: 0, width: 1280, height: 720, isPrimary: false })
  );
  return `${window.location.pathname}?${buildPopupQuery(placeholder, monitorIndex)}`;
}

function parsePopupConfig(search: string): {
  monitors: MonitorSpec[];
  monitorIndex: number;
} | null {
  const params = new URLSearchParams(search);
  if (params.get('popup') !== '1') return null;
  const monitorIndex = Number(params.get('monitorIndex'));
  const encoded = params.get('monitors');
  if (!encoded || !Number.isFinite(monitorIndex)) return null;
  try {
    const monitors = JSON.parse(atob(encoded)) as MonitorSpec[];
    if (!Array.isArray(monitors) || monitors.length === 0) return null;
    if (monitorIndex < 0 || monitorIndex >= monitors.length) return null;
    return { monitors, monitorIndex };
  } catch {
    return null;
  }
}
