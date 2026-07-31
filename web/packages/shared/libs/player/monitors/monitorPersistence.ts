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
 * Persists the multi-monitor arrangement to `localStorage` so it can be offered
 * for one-click restore on the next connection. We save each window's exact
 * physical rect (top-left + size), which is what lets a "two monitors on one
 * screen" arrangement (e.g. left/right halves) round-trip — the popups reopen
 * via `window.open` at those rects.
 *
 * Lifecycle (see `DesktopSessionTestMulti`): the live layout is saved only while
 * it has at least one popup; a single-monitor session clears the entry. So if
 * the user connects and never restores, the previous layout is forgotten.
 */
import type { ModelMonitor } from './monitorModel';
import type { ArrangementMode } from './monitorSession';

const VERSION = 1 as const;

/** One persisted monitor. Geometry is CSS px; `physical` is OS screen coords. */
export interface SavedMonitor {
  role: 'main' | 'popup';
  isPrimary: boolean;
  physical: { left: number; top: number } | null;
  width: number;
  height: number;
  manualOffset: { x: number; y: number } | null;
}

export interface SavedLayout {
  version: typeof VERSION;
  arrangement: ArrangementMode;
  monitors: SavedMonitor[];
}

/** Snapshot the model's monitors into a serializable layout. */
export function serializeLayout(
  monitors: ModelMonitor[],
  arrangement: ArrangementMode
): SavedLayout {
  return {
    version: VERSION,
    arrangement,
    monitors: monitors.map(m => ({
      role: m.role,
      isPrimary: m.isPrimary,
      physical: m.physical
        ? { left: m.physical.left, top: m.physical.top }
        : null,
      width: m.cssWidth,
      height: m.cssHeight,
      manualOffset: m.manualOffset
        ? { x: m.manualOffset.x, y: m.manualOffset.y }
        : null,
    })),
  };
}

/** Number of popup (non-main) monitors — i.e. windows a restore would reopen. */
export function popupCount(layout: SavedLayout): number {
  return layout.monitors.filter(m => m.role === 'popup').length;
}

/** Read + validate a saved layout. Returns null on absent/corrupt/old data. */
export function loadLayout(key: string): SavedLayout | null {
  let raw: string | null;
  try {
    raw = localStorage.getItem(key);
  } catch {
    return null; // localStorage unavailable (SSR / privacy mode)
  }
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return isValidLayout(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function saveLayout(key: string, layout: SavedLayout): void {
  try {
    localStorage.setItem(key, JSON.stringify(layout));
  } catch {
    /* quota exceeded / unavailable — non-fatal */
  }
}

export function clearLayout(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    /* unavailable — non-fatal */
  }
}

function isPoint(x: unknown, a: string, b: string): boolean {
  if (!x || typeof x !== 'object') return false;
  const p = x as Record<string, unknown>;
  return typeof p[a] === 'number' && typeof p[b] === 'number';
}

function isValidMonitor(x: unknown): boolean {
  if (!x || typeof x !== 'object') return false;
  const m = x as Record<string, unknown>;
  if (m.role !== 'main' && m.role !== 'popup') return false;
  if (typeof m.isPrimary !== 'boolean') return false;
  if (typeof m.width !== 'number' || typeof m.height !== 'number') return false;
  if (m.physical !== null && !isPoint(m.physical, 'left', 'top')) return false;
  if (m.manualOffset !== null && !isPoint(m.manualOffset, 'x', 'y'))
    return false;
  return true;
}

function isValidLayout(x: unknown): x is SavedLayout {
  if (!x || typeof x !== 'object') return false;
  const l = x as Record<string, unknown>;
  if (l.version !== VERSION) return false;
  if (l.arrangement !== 'auto' && l.arrangement !== 'manual') return false;
  if (!Array.isArray(l.monitors)) return false;
  return l.monitors.every(isValidMonitor);
}
