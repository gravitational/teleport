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

import { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { Flex, Text } from 'design';
import { Desktop } from 'design/Icon';

/** How long the bar stays up once the session is connected (and after a
 * manual reveal) before sliding away to the slim reveal tab. */
const AUTO_HIDE_MS = 4000;

/** Pointer within this distance of the pill counts as "heading for the bar"
 * and holds off the auto-hide — otherwise the bar can vanish mid-approach
 * right as the user goes to click it on initial launch. */
const NEAR_PX = 120;

interface MonitorTaskbarProps {
  monitorCount: number;
  maxMonitors: number;
  fps?: number | null;
  cpu?: number | null;
  statusLabel?: string;
  onOpenMonitors: () => void;
  /** Enter pointer-lock capture mode (seamless cross-monitor input). */
  onCaptureInput?: () => void;
  /** Pointer lock is actually held: the whole overlay disappears (it isn't
   * clickable under the lock anyway, and it would pollute recordings). */
  captureActive?: boolean;
  /** Capture is engaged but the browser dropped the lock (bare Esc). Shows
   * the "paused" pill so the state is visible and escapable — without it the
   * session looks normal while clicks re-lock and the bar stays hidden. */
  capturePaused?: boolean;
  /** Re-acquire the lock without recentering the virtual cursor. */
  onResumeCapture?: () => void;
  /** Fully exit capture (same as the Ctrl+Alt+Shift+Esc chord). */
  onExitCapture?: () => void;
  /** Saved popup monitors available to restore (0 hides the restore button). */
  restorable?: number;
  /** Reopen the saved monitor layout (one-click restore from a user gesture). */
  onRestore?: () => void;
  /** Toggle the session DPR between 1x and the device DPR mid-stream (AVC probe). */
  onToggleHidpi?: () => void;
  /** Whether HiDPI (device-DPR) is currently active. */
  hidpiActive?: boolean;
}

/**
 * Slim floating toolbar pinned to the top of the main session window. Built as
 * a self-contained overlay for the codec-test harness; the "Monitors" control
 * is shaped to drop into the production `TopBar`/`renderControls` later.
 *
 * It floats (doesn't reserve layout space) so it never changes the RDP
 * resolution; `pointer-events` are scoped to the pill so the rest of the canvas
 * stays interactive.
 */
export function MonitorTaskbar({
  monitorCount,
  maxMonitors,
  fps,
  cpu,
  statusLabel,
  onOpenMonitors,
  onCaptureInput,
  captureActive,
  restorable = 0,
  onRestore,
  onToggleHidpi,
  hidpiActive,
  capturePaused,
  onResumeCapture,
  onExitCapture,
}: MonitorTaskbarProps) {
  // The bar overlays the remote desktop, so it auto-hides shortly after the
  // session connects, leaving only the slim reveal tab. Hovering holds it
  // open; pre-open states (statusLabel set) pin it; leaving capture mode
  // re-reveals it briefly so the state change is visible.
  const [revealed, setRevealed] = useState(true);
  const [hovered, setHovered] = useState(false);
  // Proximity watch: the bar floats with pointer-events scoped to the pill,
  // so an enlarged DOM hitbox would steal canvas input — instead measure the
  // pointer's distance to the pill on (passive) window mousemoves. `near`
  // holds off the auto-hide while the pointer is heading for the bar.
  const pillRef = useRef<HTMLDivElement | null>(null);
  const [near, setNear] = useState(false);
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const el = pillRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const dx = Math.max(r.left - e.clientX, 0, e.clientX - r.right);
      const dy = Math.max(r.top - e.clientY, 0, e.clientY - r.bottom);
      // Same-value sets are no-op re-renders; React bails out.
      setNear(dx <= NEAR_PX && dy <= NEAR_PX);
    };
    window.addEventListener('mousemove', onMove, { passive: true });
    return () => window.removeEventListener('mousemove', onMove);
  }, []);
  useEffect(() => {
    if (statusLabel || capturePaused) {
      setRevealed(true);
      return;
    }
    if (!revealed || hovered || near) return;
    const t = setTimeout(() => setRevealed(false), AUTO_HIDE_MS);
    return () => clearTimeout(t);
  }, [statusLabel, capturePaused, revealed, hovered, near]);
  useEffect(() => {
    // Any transition back toward normal (lock dropped, capture exited)
    // re-reveals the bar so the state change is visible.
    if (!captureActive) setRevealed(true);
  }, [captureActive, capturePaused]);

  // Pointer lock held: no overlay at all — nothing here is clickable under
  // the lock, and chrome over the desktop would pollute the recording.
  if (captureActive) return null;

  // Lock dropped while capture is engaged (bare Esc): the session LOOKS
  // normal but clicks/keys re-lock, so say so and offer the exits.
  if (capturePaused) {
    return (
      <Bar>
        <Pill>
          <Metric>input capture paused</Metric>
          <CaptureButton
            onClick={onResumeCapture}
            data-active
            title="Re-acquire the pointer (clicking the canvas or typing also resumes)"
          >
            <Text typography="body3">Resume</Text>
          </CaptureButton>
          <CaptureButton
            onClick={onExitCapture}
            title="Leave capture mode (Ctrl+Alt+Shift+Esc)"
          >
            <Text typography="body3">Exit capture</Text>
          </CaptureButton>
        </Pill>
      </Bar>
    );
  }

  if (!revealed) {
    return (
      <Bar>
        <RevealTab
          onClick={() => setRevealed(true)}
          title="Show session toolbar"
        >
          <Desktop size="small" />
        </RevealTab>
      </Bar>
    );
  }

  return (
    <Bar>
      <Pill
        ref={pillRef}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        {restorable > 0 && onRestore && (
          <CaptureButton
            onClick={onRestore}
            data-active
            title="Reopen the monitors from your previous session"
          >
            <Text typography="body3">
              Restore {restorable} monitor{restorable === 1 ? '' : 's'}
            </Text>
          </CaptureButton>
        )}
        <MonitorsButton onClick={onOpenMonitors} title="Manage monitors">
          <Desktop size="small" />
          <Text typography="body3" ml={2}>
            Monitors
          </Text>
          <Count>
            {monitorCount}/{maxMonitors}
          </Count>
        </MonitorsButton>
        {onCaptureInput && (
          <CaptureButton
            onClick={onCaptureInput}
            data-active={!!captureActive}
            title="Capture input for seamless cross-monitor pointer (Ctrl+Alt+Shift+Esc to release)"
          >
            <Text typography="body3">
              {captureActive ? 'Captured' : 'Capture input'}
            </Text>
          </CaptureButton>
        )}
        {onToggleHidpi && (
          <CaptureButton
            onClick={onToggleHidpi}
            data-active={!!hidpiActive}
            title="Toggle session DPR between 1x and device HiDPI mid-stream (watch the node log for AVC)"
          >
            <Text typography="body3">
              {hidpiActive ? 'HiDPI on' : 'HiDPI off'}
            </Text>
          </CaptureButton>
        )}
        {(fps != null || cpu != null) && (
          <Metric>
            {fps != null ? `${Math.round(fps)} fps` : ''}
            {cpu != null ? ` · ${Math.round(cpu)}% cpu` : ''}
          </Metric>
        )}
        {statusLabel && <Metric>{statusLabel}</Metric>}
      </Pill>
    </Bar>
  );
}

const Bar = styled.div`
  position: fixed;
  top: 8px;
  left: 0;
  right: 0;
  display: flex;
  justify-content: center;
  pointer-events: none;
  z-index: 5;
`;

const Pill = styled(Flex)`
  pointer-events: auto;
  align-items: center;
  gap: 12px;
  background: rgba(12, 20, 61, 0.85);
  backdrop-filter: blur(6px);
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 999px;
  padding: 4px 8px 4px 4px;
  color: ${p => p.theme.colors.text.main};
`;

const RevealTab = styled.button`
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  background: rgba(12, 20, 61, 0.55);
  color: ${p => p.theme.colors.text.slightlyMuted};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 999px;
  padding: 2px 10px;
  cursor: pointer;
  opacity: 0.6;
  &:hover {
    opacity: 1;
  }
`;

const MonitorsButton = styled.button`
  display: inline-flex;
  align-items: center;
  gap: 2px;
  background: ${p => p.theme.colors.interactive.tonal.primary[1]};
  color: ${p => p.theme.colors.text.main};
  border: none;
  border-radius: 999px;
  padding: 6px 10px;
  cursor: pointer;
  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.primary[2]};
  }
`;

const CaptureButton = styled.button`
  display: inline-flex;
  align-items: center;
  background: ${p => p.theme.colors.spotBackground[0]};
  color: ${p => p.theme.colors.text.main};
  border: none;
  border-radius: 999px;
  padding: 6px 10px;
  cursor: pointer;
  &:hover {
    background: ${p => p.theme.colors.spotBackground[1]};
  }
  &[data-active='true'] {
    background: ${p => p.theme.colors.interactive.solid.success.default};
    color: ${p => p.theme.colors.text.primaryInverse};
  }
`;

const Count = styled.span`
  margin-left: 8px;
  font-size: 12px;
  font-weight: 700;
  background: ${p => p.theme.colors.interactive.solid.primary.default};
  color: ${p => p.theme.colors.text.primaryInverse};
  border-radius: 999px;
  padding: 1px 8px;
`;

const Metric = styled.span`
  font-family: monospace;
  font-size: 12px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  padding-right: 6px;
`;
