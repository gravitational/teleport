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

import { useCallback, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, H3, Text, Toggle } from 'design';
import { Cross, Desktop, Plus, Stars, Trash } from 'design/Icon';

import {
  clampRectToDisplays,
  snapRect,
  snapZoneForPointer,
} from './monitorLayout';
import type {
  ManagedMonitorView,
  MonitorSession,
  MonitorSessionState,
} from './monitorSession';
import type { DisplayInfo, ScreenTopology } from './useScreenTopology';

interface MonitorManagerPanelProps {
  open: boolean;
  onClose: () => void;
  session: MonitorSession;
  state: MonitorSessionState;
  topology: ScreenTopology;
}

// Sized so a 4-wide horizontal monitor row still leaves room to drag tiles
// around (the world is fit-scaled into this box).
const MAP_W = 1080;
const MAP_H = 480;
const MAP_PAD = 28;
/** Snap pull distance while dragging a tile, in panel px (converted to world
 * px at the current map scale). */
const SNAP_PANEL_PX = 12;
/** Edge-band depth (fraction of the display dimension) that triggers
 * half/quarter zones while DRAGGING a monitor — narrow, so dragging across
 * the map doesn't constantly offer resizes. */
const DRAG_ZONE_BAND = 0.15;
/** Band depth for the hover-to-ADD zones — a 3×3 grid of generous regions
 * (corner ninths → quarters, edge ninths → halves, center → whole display). */
const ADD_ZONE_BAND = 1 / 3;

/** Usable area of a display (no availWidth/Height in DisplayInfo — assume
 * the dock/menubar inset is on the leading edges avail* reflects). */
function displayAvailRect(s: DisplayInfo): Rect {
  return {
    left: s.availLeft,
    top: s.availTop,
    width: s.width - (s.availLeft - s.left),
    height: s.height - (s.availTop - s.top),
  };
}

interface Rect {
  left: number;
  top: number;
  width: number;
  height: number;
}

/** Uniform fit transform from world (OS virtual-screen) coords → panel px. */
function fitTransform(rects: Rect[]) {
  if (rects.length === 0) {
    return { scale: 1, ox: 0, oy: 0 };
  }
  const minX = Math.min(...rects.map(r => r.left));
  const minY = Math.min(...rects.map(r => r.top));
  const maxX = Math.max(...rects.map(r => r.left + r.width));
  const maxY = Math.max(...rects.map(r => r.top + r.height));
  const worldW = Math.max(1, maxX - minX);
  const worldH = Math.max(1, maxY - minY);
  const scale = Math.min(
    (MAP_W - MAP_PAD * 2) / worldW,
    (MAP_H - MAP_PAD * 2) / worldH
  );
  // Center the world within the map area.
  const ox = MAP_PAD + (MAP_W - MAP_PAD * 2 - worldW * scale) / 2 - minX * scale;
  const oy = MAP_PAD + (MAP_H - MAP_PAD * 2 - worldH * scale) / 2 - minY * scale;
  return { scale, ox, oy };
}

/** World rect for a monitor: physical window box if known, else its layout rect. */
function monitorWorldRect(m: ManagedMonitorView): Rect {
  if (m.physical) {
    return {
      left: m.physical.left,
      top: m.physical.top,
      width: m.cssWidth,
      height: m.cssHeight,
    };
  }
  return { left: m.rect.x, top: m.rect.y, width: m.rect.width, height: m.rect.height };
}

export function MonitorManagerPanel({
  open,
  onClose,
  session,
  state,
  topology,
}: MonitorManagerPanelProps) {
  const [selected, setSelected] = useState<number | null>(null);
  const dragRef = useRef<{
    id: number;
    startX: number;
    startY: number;
    // World (virtual-screen content) rect of the window at drag start.
    baseLeft: number;
    baseTop: number;
    width: number;
    height: number;
    // Snap targets, captured at drag start: the other monitors + displays.
    anchors: Rect[];
  } | null>(null);
  // rAF-throttle the real `window.moveTo` calls: pointermove can fire at
  // 120Hz+ and moving an OS window is not free.
  const pendingMoveRef = useRef<{ left: number; top: number } | null>(null);
  const moveRafRef = useRef(0);
  // Active half/quarter snap zone while dragging (world coords). State for
  // the translucent preview; ref for the pointerup apply.
  const [zonePreview, setZonePreview] = useState<Rect | null>(null);
  const zoneRef = useRef<Rect | null>(null);
  const mapRef = useRef<HTMLDivElement | null>(null);
  // Hover-to-add: pointer over free space on a display offers a snapped zone
  // (center = whole display, edges = halves, corners = quarters); clicking
  // opens a monitor there.
  const [addPreview, setAddPreview] = useState<{
    zone: Rect;
    display: DisplayInfo;
  } | null>(null);

  const atCap = state.monitors.length >= state.maxMonitors;

  // Build the world rects (displays + monitor windows) and the fit transform.
  const displayRects: Rect[] = topology.screens.map(s => ({
    left: s.left,
    top: s.top,
    width: s.width,
    height: s.height,
  }));
  const monitorRects = state.monitors.map(monitorWorldRect);
  const { scale, ox, oy } = fitTransform([...displayRects, ...monitorRects]);

  // Dragging a popup tile moves the ACTUAL browser window (`moveTo` works on
  // script-opened windows); the position tracker then feeds the move back
  // into the layout, closing the loop. The main browser window can't be
  // moved by script, so its tile is the fixed anchor — move that OS window
  // by hand instead.
  const onTilePointerDown = useCallback(
    (m: ManagedMonitorView) => (e: React.PointerEvent) => {
      e.preventDefault();
      setSelected(m.id);
      if (m.role === 'main') return;
      const world = monitorWorldRect(m);
      (e.target as Element).setPointerCapture?.(e.pointerId);
      // Hold server layout pushes for the whole drag — committed on release.
      session.beginMonitorInteraction();
      dragRef.current = {
        id: m.id,
        startX: e.clientX,
        startY: e.clientY,
        baseLeft: world.left,
        baseTop: world.top,
        width: world.width,
        height: world.height,
        anchors: [
          ...displayRects,
          ...state.monitors
            .filter(o => o.id !== m.id)
            .map(monitorWorldRect),
        ],
      };
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [state.monitors, topology.screens, session]
  );

  const onTilePointerMove = useCallback(
    (e: React.PointerEvent) => {
      const d = dragRef.current;
      const map = mapRef.current;
      if (!d || !map || scale === 0) return;
      // OS-style snap zones first: pointer near a display edge/corner offers
      // a half/quarter of that display (applied with a RESIZE on release,
      // like dropping a window against a screen edge in Windows/Magnet).
      const mapBox = map.getBoundingClientRect();
      const wx = (e.clientX - mapBox.left - ox) / scale;
      const wy = (e.clientY - mapBox.top - oy) / scale;
      let zone: Rect | null = null;
      for (const s of topology.screens) {
        const z = snapZoneForPointer(wx, wy, displayAvailRect(s), DRAG_ZONE_BAND);
        if (z) {
          zone = z;
          break;
        }
      }
      zoneRef.current = zone;
      setZonePreview(zone);
      // The window itself keeps following the cursor (position-snapped flush
      // against the other monitors/displays); the zone applies on release.
      // Clamped so a monitor can never be dragged off the real displays.
      const snapped = snapRect(
        {
          left: Math.round(d.baseLeft + (e.clientX - d.startX) / scale),
          top: Math.round(d.baseTop + (e.clientY - d.startY) / scale),
          width: d.width,
          height: d.height,
        },
        d.anchors,
        SNAP_PANEL_PX / scale
      );
      pendingMoveRef.current = clampRectToDisplays(
        { ...snapped, width: d.width, height: d.height },
        topology.screens.map(displayAvailRect)
      );
      if (!moveRafRef.current) {
        moveRafRef.current = requestAnimationFrame(() => {
          moveRafRef.current = 0;
          const pos = pendingMoveRef.current;
          const drag = dragRef.current;
          if (pos && drag) session.moveMonitorWindow(drag.id, pos);
        });
      }
    },
    [scale, ox, oy, session, topology.screens]
  );

  const onTilePointerUp = useCallback(
    (e: React.PointerEvent) => {
      const d = dragRef.current;
      if (!d) return;
      (e.target as Element).releasePointerCapture?.(e.pointerId);
      const zone = zoneRef.current;
      if (zone) {
        // Drop into the zone: move + resize the real window, OS-snap style.
        session.setMonitorWindowBounds(d.id, zone);
      } else {
        // Flush the final position before ending the drag.
        const pos = pendingMoveRef.current;
        if (pos) session.moveMonitorWindow(d.id, pos);
      }
      zoneRef.current = null;
      setZonePreview(null);
      pendingMoveRef.current = null;
      dragRef.current = null;
      session.endMonitorInteraction();
    },
    [session]
  );

  const onMapMouseMove = useCallback(
    (e: React.MouseEvent) => {
      const map = mapRef.current;
      if (!map || scale === 0 || dragRef.current || atCap) {
        setAddPreview(null);
        return;
      }
      const box = map.getBoundingClientRect();
      const wx = (e.clientX - box.left - ox) / scale;
      const wy = (e.clientY - box.top - oy) / scale;
      // Over an existing monitor: that's drag/select territory, not add.
      for (const m of state.monitors) {
        const r = monitorWorldRect(m);
        if (
          wx >= r.left &&
          wx <= r.left + r.width &&
          wy >= r.top &&
          wy <= r.top + r.height
        ) {
          setAddPreview(null);
          return;
        }
      }
      for (const s of topology.screens) {
        const avail = displayAvailRect(s);
        const inside =
          wx >= avail.left &&
          wx <= avail.left + avail.width &&
          wy >= avail.top &&
          wy <= avail.top + avail.height;
        if (!inside) continue;
        const zone =
          snapZoneForPointer(wx, wy, avail, ADD_ZONE_BAND) ?? avail; // center ninth → the whole display
        // Don't offer a spot that's already (mostly) occupied — with the
        // primary filling the left half of a display, only the right-side
        // zones remain. The 10% tolerance keeps a window slightly wider
        // than half from blocking the other half.
        const zoneArea = zone.width * zone.height;
        const occupied = state.monitors.some(m => {
          const r = monitorWorldRect(m);
          const ix =
            Math.min(zone.left + zone.width, r.left + r.width) -
            Math.max(zone.left, r.left);
          const iy =
            Math.min(zone.top + zone.height, r.top + r.height) -
            Math.max(zone.top, r.top);
          return ix > 0 && iy > 0 && ix * iy > zoneArea * 0.1;
        });
        if (occupied) {
          setAddPreview(null);
          return;
        }
        setAddPreview({ zone, display: s });
        return;
      }
      setAddPreview(null);
    },
    [scale, ox, oy, atCap, state.monitors, topology.screens]
  );

  const onMapClick = useCallback(() => {
    if (!addPreview || atCap) return;
    // Must run synchronously in the click gesture: window.open is
    // popup-blocked otherwise.
    session.addMonitor(addPreview.display, addPreview.zone);
    setAddPreview(null);
  }, [addPreview, atCap, session]);

  if (!open) return null;

  const selectedMonitor = state.monitors.find(m => m.id === selected) ?? null;

  return (
    <Backdrop onMouseDown={onClose}>
      <Panel onMouseDown={e => e.stopPropagation()}>
        <Flex justifyContent="space-between" alignItems="center" mb={2}>
          <H3>Monitor layout</H3>
          <Flex alignItems="center" gap={3}>
            <Flex alignItems="center" gap={2}>
              <Text typography="body3" color="text.slightlyMuted">
                Auto-arrange
              </Text>
              <Toggle
                isToggled={state.arrangement === 'auto'}
                onToggle={() =>
                  session.setArrangement(
                    state.arrangement === 'auto' ? 'manual' : 'auto'
                  )
                }
              />
            </Flex>
            <CloseButton aria-label="Close" onClick={onClose}>
              <Cross size="small" />
            </CloseButton>
          </Flex>
        </Flex>

        {topology.permission !== 'granted' && topology.supported && (
          <Notice>
            <Text typography="body3">
              Grant display access to see where your monitors are in physical
              space and place new ones precisely.
            </Text>
            <ButtonSecondary
              size="small"
              onClick={() => void topology.requestPermission()}
            >
              Grant access
            </ButtonSecondary>
          </Notice>
        )}
        {!topology.supported && (
          <Notice>
            <Text typography="body3">
              This browser doesn't support the Window Management API, so the
              physical-display map is unavailable. Arrangement still tracks
              window positions.
            </Text>
          </Notice>
        )}

        <MapArea
          ref={mapRef}
          style={{
            width: MAP_W,
            height: MAP_H,
            // Clickable add-zone under the pointer.
            cursor: addPreview ? 'pointer' : undefined,
          }}
          onMouseMove={onMapMouseMove}
          onMouseLeave={() => setAddPreview(null)}
          onClick={onMapClick}
        >
          {/* Faint physical displays behind the monitor tiles. */}
          {topology.screens.map(s => (
            <DisplayRect
              key={s.id}
              style={{
                left: s.left * scale + ox,
                top: s.top * scale + oy,
                width: s.width * scale,
                height: s.height * scale,
              }}
            >
              <DisplayLabel>
                {s.label}
                {s.isPrimary ? ' • primary display' : ''}
              </DisplayLabel>
            </DisplayRect>
          ))}

          {/* Active monitor windows as tiles. Popups drag the real window;
              the main tile is the fixed anchor. */}
          {state.monitors.map((m, i) => {
            const r = monitorWorldRect(m);
            const isSel = m.id === selected;
            const tileW = Math.max(36, r.width * scale);
            const tileH = Math.max(28, r.height * scale);
            return (
              <Tile
                key={m.id}
                data-selected={isSel}
                data-status={m.status}
                data-fixed={m.role === 'main'}
                title={
                  m.role === 'main'
                    ? 'The main browser window anchors the layout — move the window itself to reposition it'
                    : 'Drag to move this monitor window on your screen'
                }
                onPointerDown={onTilePointerDown(m)}
                onPointerMove={onTilePointerMove}
                onPointerUp={onTilePointerUp}
                onPointerCancel={onTilePointerUp}
                style={{
                  left: r.left * scale + ox,
                  top: r.top * scale + oy,
                  width: tileW,
                  height: tileH,
                }}
              >
                <TileIndex>{i + 1}</TileIndex>
                {m.isPrimary && (
                  <PrimaryBadge title="Primary monitor">
                    <Stars size="small" />
                  </PrimaryBadge>
                )}
                {/* Meta only when it can fit inside the tile — on small
                    tiles it would spill over neighboring labels. */}
                {tileW >= 96 && (
                  <TileMeta>
                    {m.cssWidth}×{m.cssHeight}
                    {m.status === 'pending' ? ' • opening…' : ''}
                    {m.status === 'blocked' ? ' • blocked' : ''}
                  </TileMeta>
                )}
              </Tile>
            );
          })}

          {/* Half/quarter snap-zone preview while dragging, like the OS's
              translucent snap outline. */}
          {zonePreview && (
            <ZonePreview
              style={{
                left: zonePreview.left * scale + ox,
                top: zonePreview.top * scale + oy,
                width: zonePreview.width * scale,
                height: zonePreview.height * scale,
              }}
            />
          )}

          {/* Hover-to-add zone: click opens a monitor in this spot. */}
          {addPreview && !zonePreview && (
            <AddZonePreview
              style={{
                left: addPreview.zone.left * scale + ox,
                top: addPreview.zone.top * scale + oy,
                width: addPreview.zone.width * scale,
                height: addPreview.zone.height * scale,
              }}
            >
              <Plus size="small" />
              <Text typography="body3">Add monitor</Text>
            </AddZonePreview>
          )}
        </MapArea>

        {state.overlaps.length > 0 && (
          <Text typography="body4" color="text.muted" mt={1}>
            Overlapping monitors were nudged apart to form a valid layout.
          </Text>
        )}

        {/* Selected-monitor actions. */}
        {selectedMonitor && (
          <Flex gap={2} mt={3} alignItems="center">
            <Text typography="body3" color="text.slightlyMuted">
              Monitor{' '}
              {state.monitors.findIndex(m => m.id === selectedMonitor.id) + 1}
            </Text>
            <ButtonSecondary
              size="small"
              disabled={selectedMonitor.isPrimary}
              onClick={() => session.setPrimary(selectedMonitor.id)}
            >
              <Stars size="small" mr={1} /> Make primary
            </ButtonSecondary>
            <ButtonSecondary
              size="small"
              disabled={selectedMonitor.role === 'main'}
              onClick={() => {
                session.removeMonitor(selectedMonitor.id);
                setSelected(null);
              }}
            >
              <Trash size="small" mr={1} /> Remove
            </ButtonSecondary>
          </Flex>
        )}

        {/* Add-monitor controls + footer. */}
        <Flex justifyContent="space-between" alignItems="flex-end" mt={3} gap={3}>
          <Box>
            <Text typography="body4" color="text.muted" mb={1}>
              Add a monitor {atCap && `(max ${state.maxMonitors} reached)`}
            </Text>
            <Flex gap={2} flexWrap="wrap">
              {topology.screens.length > 0 ? (
                topology.screens.map(s => (
                  <AddButton
                    key={s.id}
                    disabled={atCap}
                    display={s}
                    onAdd={() => session.addMonitor(s)}
                  />
                ))
              ) : (
                <ButtonSecondary
                  size="small"
                  disabled={atCap}
                  onClick={() => session.addMonitor()}
                >
                  <Plus size="small" mr={1} /> New monitor
                </ButtonSecondary>
              )}
            </Flex>
          </Box>
          <Flex gap={2} alignItems="center" flexShrink={0}>
            <Text
              typography="body3"
              color="text.slightlyMuted"
              style={{ whiteSpace: 'nowrap' }}
            >
              {state.monitors.length}/{state.maxMonitors} monitors
            </Text>
            <ButtonPrimary size="small" onClick={onClose}>
              Done
            </ButtonPrimary>
          </Flex>
        </Flex>
      </Panel>
    </Backdrop>
  );
}

function AddButton({
  display,
  disabled,
  onAdd,
}: {
  display: DisplayInfo;
  disabled: boolean;
  onAdd: () => void;
}) {
  return (
    <ButtonSecondary size="small" disabled={disabled} onClick={onAdd}>
      <Desktop size="small" mr={1} /> {display.label}
    </ButtonSecondary>
  );
}

const Backdrop = styled.div`
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
`;

const Panel = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  color: ${p => p.theme.colors.text.main};
  border-radius: 12px;
  padding: 20px 24px;
  width: 1128px;
  max-width: 94vw;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.4);
`;

const CloseButton = styled.button`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: ${p => p.theme.colors.text.slightlyMuted};
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
    color: ${p => p.theme.colors.text.main};
  }
`;

const Notice = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: ${p => p.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 10px 14px;
  margin-bottom: 12px;
`;

const MapArea = styled.div`
  position: relative;
  background: ${p => p.theme.colors.levels.sunken};
  border-radius: 10px;
  overflow: hidden;
  user-select: none;
  touch-action: none;
`;

const DisplayRect = styled.div`
  position: absolute;
  border: 1px dashed ${p => p.theme.colors.spotBackground[2]};
  border-radius: 6px;
  background: ${p => p.theme.colors.spotBackground[0]};
`;

const DisplayLabel = styled.div`
  position: absolute;
  left: 6px;
  right: 6px;
  bottom: 4px;
  font-size: 10px;
  color: ${p => p.theme.colors.text.muted};
  /* Clip to the display rect — a long label would otherwise run over the
     neighboring display's label. */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const Tile = styled.div`
  position: absolute;
  border-radius: 8px;
  border: 2px solid transparent;
  background: ${p => p.theme.colors.interactive.tonal.primary[2]};
  cursor: grab;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  &:active {
    cursor: grabbing;
  }
  &[data-selected='true'] {
    border-color: ${p => p.theme.colors.interactive.solid.primary.default};
    background: ${p => p.theme.colors.interactive.tonal.primary[2]};
  }
  &[data-status='pending'] {
    opacity: 0.6;
    border-style: dashed;
  }
  &[data-status='blocked'] {
    background: ${p => p.theme.colors.interactive.tonal.danger[1]};
  }
  &[data-fixed='true'] {
    cursor: default;
  }
`;

const TileIndex = styled.div`
  font-size: 16px;
  font-weight: 700;
  color: ${p => p.theme.colors.text.main};
`;

const TileMeta = styled.div`
  position: absolute;
  bottom: 4px;
  left: 4px;
  right: 4px;
  text-align: center;
  font-size: 10px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  /* Clip to the tile — small tiles otherwise spill text over neighbors. */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const ZonePreview = styled.div`
  position: absolute;
  pointer-events: none;
  border: 2px solid ${p => p.theme.colors.interactive.solid.primary.default};
  background: ${p => p.theme.colors.interactive.tonal.primary[0]};
  border-radius: 6px;
  z-index: 2;
`;

const AddZonePreview = styled.div`
  position: absolute;
  pointer-events: none;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 2px dashed ${p => p.theme.colors.interactive.solid.primary.default};
  background: ${p => p.theme.colors.interactive.tonal.primary[0]};
  border-radius: 6px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  overflow: hidden;
  white-space: nowrap;
  z-index: 1;
`;

const PrimaryBadge = styled.div`
  position: absolute;
  top: 4px;
  right: 4px;
  color: ${p => p.theme.colors.interactive.solid.alert.default};
  display: inline-flex;
`;
