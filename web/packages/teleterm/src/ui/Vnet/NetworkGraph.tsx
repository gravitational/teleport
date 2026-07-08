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
 *
 * TODO: The data is currently mocked. Wire it up to real per-second throughput
 * reported by the VNet service.
 */
export const NetworkGraph = () => {
  const samples = useMockThroughput();
  const theme = useTheme();

  const downColor = theme.colors.interactive.solid.accent.default;
  const upColor = theme.colors.interactive.solid.danger.default;

  // A single peak scales both halves symmetrically, so the zero line stays in
  // the middle and equal up/down rates reach equally far from it. Scale to the
  // largest value currently in the window so the graph stays readable
  // regardless of absolute throughput; guard against an all-zero window to
  // avoid dividing by zero.
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
      <Flex gap={3}>
        <Legend color={downColor} label="Received" value={latest.down} />
        <Legend color={upColor} label="Sent" value={latest.up} />
      </Flex>
    </Flex>
  );
};

const Legend = (props: { color: string; label: string; value: number }) => (
  <Flex alignItems="center" gap={1}>
    <svg width={8} height={8} aria-hidden>
      <circle cx={4} cy={4} r={4} fill={props.color} />
    </svg>
    <Text typography="body3" color="text.slightlyMuted">
      {props.label}: {formatThroughput(props.value)}
    </Text>
  </Flex>
);

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
    const y = center + sign * (selector(sample) / peak) * halfHeight;
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
 * useMockThroughput keeps a rolling window of fake throughput samples, adding a
 * new one on a fixed interval. It exists only until real data is available; see
 * the TODO on NetworkGraph.
 */
function useMockThroughput(): Sample[] {
  const [samples, setSamples] = useState<Sample[]>(() =>
    Array.from({ length: SAMPLE_COUNT }, () => ({ down: 0, up: 0 }))
  );
  // Keep the smoothed values between ticks so the mock series drift rather than
  // jump, which reads more like real traffic.
  const lastRef = useRef<Sample>({ down: 0, up: 0 });

  useEffect(() => {
    const interval = setInterval(() => {
      const next = nextMockSample(lastRef.current);
      lastRef.current = next;
      setSamples(prev => [...prev.slice(1), next]);
    }, SAMPLE_INTERVAL_MS);
    return () => clearInterval(interval);
  }, []);

  return samples;
}

/**
 * nextMockSample produces a plausible-looking next throughput sample by nudging
 * the previous one and occasionally spiking, so the graph has visible motion.
 */
function nextMockSample(prev: Sample): Sample {
  const drift = (value: number, ceiling: number) => {
    const spike = Math.random() < 0.15 ? ceiling * Math.random() : 0;
    const wander = (Math.random() - 0.5) * ceiling * 0.4;
    return Math.max(0, Math.min(ceiling, value * 0.7 + wander + spike));
  };
  return {
    // Downlink is typically heavier than uplink.
    down: drift(prev.down, 5 * 1024 * 1024),
    up: drift(prev.up, 1024 * 1024),
  };
}

/** formatThroughput renders a bytes-per-second rate in a compact, human form. */
function formatThroughput(bytesPerSecond: number): string {
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = bytesPerSecond;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  const rounded =
    value >= 100 || unit === 0 ? Math.round(value) : value.toFixed(1);
  return `${rounded} ${units[unit]}/s`;
}
