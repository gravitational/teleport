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

import { useCallback, useEffect, useRef, useState } from 'react';

import { contentScreenPosition } from './monitorLayout';

// --- Window Management API typings (lib.dom.d.ts doesn't ship these in all
// toolchains, so we declare the subset we use). ---

export interface ScreenDetailed extends Screen, EventTarget {
  readonly availLeft: number;
  readonly availTop: number;
  readonly left: number;
  readonly top: number;
  readonly isPrimary: boolean;
  readonly isInternal: boolean;
  readonly label: string;
  readonly devicePixelRatio: number;
}

export interface ScreenDetails extends EventTarget {
  readonly screens: ScreenDetailed[];
  readonly currentScreen: ScreenDetailed;
}

type WindowWithScreenDetails = Window & {
  getScreenDetails?: () => Promise<ScreenDetails>;
};

/** Normalized, framework-agnostic view of one physical display. */
export interface DisplayInfo {
  /** Stable-ish id derived from label + geometry (no native id exists). */
  id: string;
  label: string;
  left: number;
  top: number;
  width: number;
  height: number;
  availLeft: number;
  availTop: number;
  isPrimary: boolean;
  isInternal: boolean;
  devicePixelRatio: number;
}

/** Live placement of a tracked window in OS virtual-screen coords (CSS px). */
export interface WindowPlacement {
  left: number;
  top: number;
  width: number;
  height: number;
  /** Which display the window's center currently sits on, if known. */
  displayId: string | null;
}

export type TopologyPermission =
  | 'granted'
  | 'prompt'
  | 'denied'
  | 'unsupported';

export interface ScreenTopology {
  /** True when the Window Management API exists in this browser. */
  supported: boolean;
  permission: TopologyPermission;
  screens: DisplayInfo[];
  /** Prompt for / fetch screen details. Must be called from a user gesture. */
  requestPermission: () => Promise<boolean>;
  /** Map a virtual-screen point (CSS px) to the display containing it. */
  displayForPoint: (left: number, top: number) => DisplayInfo | null;
  /**
   * Watch a (same-origin) popup window's live position + size. Polls because
   * there is no window "move" event, and also listens for `resize`. Returns an
   * unsubscribe.
   */
  trackWindow: (win: Window, cb: (p: WindowPlacement) => void) => () => void;
}

function displayId(s: ScreenDetailed, index: number): string {
  // No native stable id; combine label + origin so it survives re-snapshots
  // and only changes when the display is genuinely reconfigured.
  return `${s.label || `display-${index}`}@${s.left},${s.top}`;
}

function toDisplayInfo(s: ScreenDetailed, index: number): DisplayInfo {
  return {
    id: displayId(s, index),
    label: s.label || `Display ${index + 1}`,
    left: s.left,
    top: s.top,
    width: s.width,
    height: s.height,
    availLeft: s.availLeft,
    availTop: s.availTop,
    isPrimary: s.isPrimary,
    isInternal: s.isInternal,
    devicePixelRatio: s.devicePixelRatio,
  };
}

/**
 * Pure: the display whose rectangle contains the point, or the nearest display
 * by center distance when the point lies in a gap. Exported for testing.
 */
export function findDisplayForPoint(
  screens: DisplayInfo[],
  left: number,
  top: number
): DisplayInfo | null {
  if (screens.length === 0) return null;
  for (const s of screens) {
    if (
      left >= s.left &&
      left < s.left + s.width &&
      top >= s.top &&
      top < s.top + s.height
    ) {
      return s;
    }
  }
  // Fallback: nearest by center distance (window dragged into a gap / offscreen).
  let best: DisplayInfo | null = null;
  let bestDist = Infinity;
  for (const s of screens) {
    const cx = s.left + s.width / 2;
    const cy = s.top + s.height / 2;
    const d = (cx - left) ** 2 + (cy - top) ** 2;
    if (d < bestDist) {
      bestDist = d;
      best = s;
    }
  }
  return best;
}

const POLL_MS = 300;

/**
 * React hook over the Window Management API. Degrades gracefully: when the API
 * is unavailable `supported` is false and the rest are inert. When available
 * but not yet granted, `permission` is 'prompt' and `screens` is empty until
 * `requestPermission()` succeeds.
 */
export function useScreenTopology(): ScreenTopology {
  const w = (typeof window !== 'undefined'
    ? (window as WindowWithScreenDetails)
    : undefined);
  const supported = !!w?.getScreenDetails;

  const [permission, setPermission] = useState<TopologyPermission>(
    supported ? 'prompt' : 'unsupported'
  );
  const [screens, setScreens] = useState<DisplayInfo[]>([]);
  const detailsRef = useRef<ScreenDetails | null>(null);
  const screensRef = useRef<DisplayInfo[]>([]);
  screensRef.current = screens;

  const snapshot = useCallback((details: ScreenDetails) => {
    setScreens(details.screens.map((s, i) => toDisplayInfo(s, i)));
  }, []);

  // Reflect the current permission state if the Permissions API knows it, so a
  // previously-granted session doesn't show a stale "prompt".
  useEffect(() => {
    if (!supported) return;
    let cancelled = false;
    const anyPerms = navigator.permissions as Permissions & {
      query: (d: { name: string }) => Promise<PermissionStatus>;
    };
    anyPerms
      ?.query({ name: 'window-management' })
      .then(status => {
        if (cancelled) return;
        if (status.state === 'granted') {
          // Eagerly fetch details; getScreenDetails won't re-prompt once granted.
          void requestPermissionImpl();
        } else if (status.state === 'denied') {
          setPermission('denied');
        }
      })
      .catch(() => {
        /* permission name unknown in this browser — leave as 'prompt' */
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [supported]);

  const requestPermissionImpl = useCallback(async (): Promise<boolean> => {
    if (!w?.getScreenDetails) return false;
    try {
      const details = await w.getScreenDetails();
      detailsRef.current = details;
      snapshot(details);
      setPermission('granted');
      const onChange = () => snapshot(details);
      details.addEventListener('screenschange', onChange);
      for (const s of details.screens) s.addEventListener('change', onChange);
      return true;
    } catch (e) {
      setPermission(
        e instanceof DOMException && e.name === 'NotAllowedError'
          ? 'denied'
          : 'prompt'
      );
      return false;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [snapshot]);

  // Clean up listeners on unmount.
  useEffect(() => {
    return () => {
      const details = detailsRef.current;
      if (!details) return;
      // We can't remove the exact closures here, but dropping the ref lets the
      // ScreenDetails be GC'd with its listeners once no longer referenced.
      detailsRef.current = null;
    };
  }, []);

  const displayForPoint = useCallback(
    (left: number, top: number) =>
      findDisplayForPoint(screensRef.current, left, top),
    []
  );

  const trackWindow = useCallback(
    (win: Window, cb: (p: WindowPlacement) => void) => {
      let last: WindowPlacement | null = null;
      const read = () => {
        if (win.closed) return;
        let left: number;
        let top: number;
        let width: number;
        let height: number;
        try {
          // Content (viewport) position, not the OS frame: main and popup
          // windows have different chrome heights, and the layout must align
          // the canvases, not the frames.
          ({ left, top } = contentScreenPosition(win));
          width = win.innerWidth;
          height = win.innerHeight;
        } catch {
          return; // cross-process hiccup; try again next tick
        }
        const center = findDisplayForPoint(
          screensRef.current,
          left + width / 2,
          top + height / 2
        );
        const next: WindowPlacement = {
          left,
          top,
          width,
          height,
          displayId: center?.id ?? null,
        };
        if (
          !last ||
          last.left !== next.left ||
          last.top !== next.top ||
          last.width !== next.width ||
          last.height !== next.height ||
          last.displayId !== next.displayId
        ) {
          last = next;
          cb(next);
        }
      };
      read();
      const id = setInterval(read, POLL_MS);
      let resizeTarget: Window | null = null;
      try {
        win.addEventListener('resize', read);
        resizeTarget = win;
      } catch {
        /* cross-origin or detached — polling still covers it */
      }
      return () => {
        clearInterval(id);
        try {
          resizeTarget?.removeEventListener('resize', read);
        } catch {
          /* window already gone */
        }
      };
    },
    []
  );

  return {
    supported,
    permission,
    screens,
    requestPermission: requestPermissionImpl,
    displayForPoint,
    trackWindow,
  };
}
