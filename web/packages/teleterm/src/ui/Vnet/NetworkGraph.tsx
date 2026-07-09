/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { useTheme } from 'styled-components';

import { Flex, Text } from 'design';
import { ConnectionStat } from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';

import { useVnetContext } from './vnetContext';

/** Number of samples kept in the rolling window. */
const SAMPLE_COUNT = 60;
/** How often a new throughput sample is taken, in milliseconds. */
const SAMPLE_INTERVAL_MS = 1000;
/** SVG viewBox dimensions. The graph scales to its container width via CSS. */
const VIEW_WIDTH = 300;
const VIEW_HEIGHT = 120;

interface Sample {
  /** Bytes received per second (downlink). */
  down: number;
  /** Bytes sent per second (uplink). */
  up: number;
}

/**
 * NetworkGraph renders live uplink/downlink throughput as two overlaid,
 * scrolling area charts, in the style of the Network tab in macOS Activity
 * Monitor. The most recent sample is on the right; older samples scroll off to
 * the left.
 */
export const NetworkGraph = () => {
  const { connectionStats } = useVnetContext();
  const samples = useThroughput(connectionStats);
  const theme = useTheme();

  // Cumulative bytes transferred over all targets since VNet started. Shown in
  // the legend alongside the current per-second rate. Summed as bigint to keep
  // the full uint64 range from the counters.
  const totalDown = Number(
    connectionStats.reduce((sum, s) => sum + s.bytesRx, 0n)
  );
  const totalUp = Number(
    connectionStats.reduce((sum, s) => sum + s.bytesTx, 0n)
  );

  const downColor = theme.colors.interactive.solid.accent.default;
  const upColor = theme.colors.interactive.solid.danger.default;

  // A single peak scales both halves symmetrically, so the zero line stays in
  // the middle and equal up/down rates reach equally far from it. Scale to the
  // largest value currently in the window so the graph stays readable
  // regardless of absolute throughput; guard against an all-zero window to
  // avoid dividing by zero. The peak feeds a logarithmic curve (see scale), so
  // low and high throughput stay legible at once and a single spike doesn't
  // flatten everything else against it.
  const peak = Math.max(1, ...samples.flatMap(s => [s.down, s.up])) * 1.1;

  const latest = samples[samples.length - 1];

  return (
    <Flex flexDirection="column" gap={1} width="100%">
      <svg
        viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="Network throughput"
        css={`
          width: 100%;
          height: ${VIEW_HEIGHT}px;
          display: block;
          padding-block: 10px;
          background: ${theme.colors.levels.sunken};
          border-radius: ${theme.radii[2]}px;
        `}
      >
        {/* Downlink grows up from the center, uplink grows down from it. */}
        <Area
          samples={samples}
          selector={s => s.down}
          peak={peak}
          color={downColor}
          direction="up"
        />
        <Area
          samples={samples}
          selector={s => s.up}
          peak={peak}
          color={upColor}
          direction="down"
        />
        {/* Zero line. */}
        <line
          x1={0}
          y1={VIEW_HEIGHT / 2}
          x2={VIEW_WIDTH}
          y2={VIEW_HEIGHT / 2}
          stroke={theme.colors.text.muted}
          strokeWidth={1}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <Flex gap={2}>
        <Legend
          color={downColor}
          label="Received"
          value={latest.down}
          total={totalDown}
        />
        <Legend
          color={upColor}
          label="Sent"
          value={latest.up}
          total={totalUp}
        />
      </Flex>
    </Flex>
  );
};

const Legend = (props: {
  color: string;
  label: string;
  value: number;
  total: number;
}) => (
  <Flex alignItems="center" gap={1}>
    <svg width={8} height={8} aria-hidden>
      <circle cx={4} cy={4} r={4} fill={props.color} />
    </svg>
    <Text typography="body3" color="text.slightlyMuted">
      {props.label} {formatThroughput(props.value)} · {formatBytes(props.total)}{' '}
      total
    </Text>
  </Flex>
);

/**
 * FLOOR_BYTES_PER_SEC anchors the bottom of the logarithmic scale. Throughput at
 * or below it renders as idle. It's set to 1 byte/sec so that any real traffic
 * registers: apps often transfer only a few KB spread over several seconds, i.e.
 * a few hundred bytes/sec, and those must still show up on the graph.
 */
const FLOOR_BYTES_PER_SEC = 1;

/**
 * scale maps a throughput value to a [0, 1] fraction of the graph's half-height
 * on a logarithmic curve anchored at FLOOR_BYTES_PER_SEC and topped at peak.
 * Linear scaling makes any low throughput vanish whenever a larger sample sits
 * in the window (e.g. after an iperf burst); the log curve keeps both legible.
 * Because peak tracks the window maximum, a lone small transfer still fills the
 * graph relative to itself — the exact rate is in the legend below.
 */
function scale(value: number, peak: number): number {
  if (value <= FLOOR_BYTES_PER_SEC) {
    return 0;
  }
  const top = Math.max(peak, FLOOR_BYTES_PER_SEC * 2);
  const fraction =
    (Math.log(value) - Math.log(FLOOR_BYTES_PER_SEC)) /
    (Math.log(top) - Math.log(FLOOR_BYTES_PER_SEC));
  return Math.min(1, fraction);
}

/**
 * Area draws a single filled, scrolling series inside the graph's viewBox,
 * anchored to the horizontal center (the zero line). `direction: 'up'` grows
 * the area toward the top, `'down'` toward the bottom, so two series can mirror
 * each other around the center.
 */
const Area = (props: {
  samples: Sample[];
  selector: (s: Sample) => number;
  peak: number;
  color: string;
  direction: 'up' | 'down';
}) => {
  const { samples, selector, peak, color, direction } = props;
  const stepX = VIEW_WIDTH / (SAMPLE_COUNT - 1);
  const center = VIEW_HEIGHT / 2;
  const halfHeight = VIEW_HEIGHT / 2;
  const sign = direction === 'up' ? -1 : 1;

  const points = samples.map((sample, i) => {
    const x = i * stepX;
    const y = center + sign * scale(selector(sample), peak) * halfHeight;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  });

  // Close the path along the center line so it renders as a filled area.
  const d = `M0,${center} L${points.join(' L')} L${VIEW_WIDTH},${center} Z`;

  return (
    <>
      <path d={d} fill={color} fillOpacity={0.25} />
      <polyline
        points={points.join(' ')}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
      />
    </>
  );
};

/**
 * useThroughput keeps a rolling window of total VNet throughput samples. Once a
 * second it sums the per-target per-second byte counters most recently reported
 * by the VNet service and appends the total as a new sample. Sampling on a fixed
 * interval (rather than per reported update) keeps the graph scrolling at a
 * steady cadence even while throughput is idle, when the service reports no
 * fresh stats.
 */
function useThroughput(connectionStats: ConnectionStat[]): Sample[] {
  const [samples, setSamples] = useState<Sample[]>(() =>
    Array.from({ length: SAMPLE_COUNT }, () => ({ down: 0, up: 0 }))
  );
  // Read the latest stats through a ref inside the interval so the ticker keeps
  // a stable identity yet always samples the most recently reported values.
  const statsRef = useRef(connectionStats);
  statsRef.current = connectionStats;

  useEffect(() => {
    const interval = setInterval(() => {
      const sample = statsRef.current.reduce<Sample>(
        (total, stat) => ({
          down: total.down + Number(stat.bytesRxPerSec),
          up: total.up + Number(stat.bytesTxPerSec),
        }),
        { down: 0, up: 0 }
      );
      setSamples(prev => [...prev.slice(1), sample]);
    }, SAMPLE_INTERVAL_MS);
    return () => clearInterval(interval);
  }, []);

  return samples;
}

/** formatBytes renders a byte count in a compact, human form. */
function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  const rounded =
    value >= 100 || unit === 0 ? Math.round(value) : value.toFixed(1);
  return `${rounded} ${units[unit]}`;
}

/** formatThroughput renders a bytes-per-second rate in a compact, human form. */
function formatThroughput(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`;
}
