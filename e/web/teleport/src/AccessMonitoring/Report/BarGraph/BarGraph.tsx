import { ResponsiveBar } from '@nivo/bar';
import { BarTooltipProps } from '@nivo/bar/dist/types/types';
import { timeFormat } from 'd3-time-format';
import { useMemo, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box } from 'design';
import { Info } from 'design/Icon';

import { COLORS } from 'e-teleport/AccessMonitoring/const';
import { Overview } from 'e-teleport/AccessMonitoring/Report/BarGraph/Overview';
import {
  convertResultToData,
  getTimescale,
  getXAxisTicks,
  MAX_RESULTS,
} from 'e-teleport/AccessMonitoring/Report/BarGraph/utils';
import { BarGraphConfig } from 'e-teleport/AccessMonitoring/Report/config';
import { ReportResult } from 'e-teleport/AccessMonitoring/types';

interface BarGraphProps {
  reportResult: ReportResult;
  graphConfig: BarGraphConfig;
  days: number;
}

const GraphContainer = styled.div`
  height: 400px;
`;

const Empty = styled.div`
  display: flex;
  justify-content: center;
  padding: ${p => p.theme.space[5]}px 0;
`;

const InfoContainer = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  border-radius: 7px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const formatter = timeFormat('%d %b');

let colorIndex = 0;

const colorCache = new Map<string | number, string>();

export function BarGraph(props: BarGraphProps) {
  const theme = useTheme();

  const [selected, setSelected] = useState<string | null>(null);

  const { from, today } = useMemo(() => getTimescale(props.days), [props.days]);
  const { count, counts, keys, data } = useMemo(
    () =>
      convertResultToData(
        props.reportResult,
        props.graphConfig,
        selected,
        from,
        today
      ),
    [props.reportResult, props.graphConfig, selected, from, today]
  );

  const tickValues = useMemo(
    () => getXAxisTicks(from, today, formatter),
    [from, today]
  );

  function handleSelect(id: string) {
    if (selected === id) {
      setSelected(null);
    } else {
      setSelected(id);
    }
  }

  function getColor(a: { id: string | number }) {
    const cached = colorCache.get(a.id);

    if (cached) {
      return cached;
    }

    const color = COLORS[colorIndex++ % COLORS.length];

    colorCache.set(a.id, color);

    return color;
  }

  if (props.reportResult.data.length === 0) {
    return <Empty>There are no results for this query</Empty>;
  }

  return (
    <Box pb={3}>
      {count > MAX_RESULTS && (
        <InfoContainer>
          <Info />
          This graph has too many results ({count}). Only the first{' '}
          {MAX_RESULTS} results are shown.
        </InfoContainer>
      )}

      <GraphContainer>
        <ResponsiveBar
          tooltip={createTooltipRenderer(props.graphConfig)}
          data={data}
          keys={keys}
          indexBy={props.graphConfig.indexByColumn}
          margin={{ top: 20, right: 20, bottom: 50, left: 60 }}
          padding={0.5}
          groupMode={props.graphConfig.groupMode}
          axisBottom={{
            tickSize: 0,
            tickPadding: 10,
            format: val => {
              const formatted = formatter(new Date(val));

              return tickValues.includes(formatted) ? formatted : '';
            },
          }}
          colors={getColor}
          enableLabel={false}
          axisLeft={{
            tickSize: 0,
            tickPadding: 20,
            tickValues: 5,
            tickRotation: 0,
          }}
          theme={{
            axis: {
              ticks: {
                text: {
                  fill: theme.colors.text.main,
                },
              },
            },
            legends: {
              text: {
                fill: theme.colors.text.main,
              },
            },
            grid: {
              line: {
                stroke: theme.colors.spotBackground[0],
              },
            },
          }}
        />
      </GraphContainer>

      <Overview
        counts={counts}
        colors={getColor}
        selected={selected}
        onSelect={handleSelect}
      />
    </Box>
  );
}

const Tooltip = styled.div`
  background: ${p => p.theme.colors.levels.popout};
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border-radius: 7px;
  display: flex;
  flex-direction: column;
  min-width: 140px;
`;

const TooltipDate = styled.div`
  font-family: ${p => p.theme.fonts.mono};
  font-size: 12px;
  line-height: 1;
  padding: ${p => p.theme.space[2]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const TooltipData = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  justify-content: space-between;
`;

const Details = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
`;

const Color = styled.div`
  width: 14px;
  height: 14px;
  border-radius: 4px;
`;

function convertObjectToData(
  obj: Record<string, string>,
  indexByColumn: string
) {
  const data = Object.entries(obj)
    .filter(([key]) => key !== indexByColumn)
    .map(
      ([key, value]: [string, string]) =>
        [key, parseInt(value, 10)] as [string, number]
    );

  // sort data by the second value

  data.sort((a, b) => b[1] - a[1]);

  return data;
}

function createTooltipRenderer(graphConfig: BarGraphConfig) {
  return function renderTooltip(props: BarTooltipProps<any>) {
    const data = useMemo(
      () => convertObjectToData(props.data, graphConfig.indexByColumn),
      [props.data, graphConfig.indexByColumn]
    );

    const items = data.map(([key, value]) => (
      <TooltipData key={key}>
        <Details>
          <Color style={{ backgroundColor: colorCache.get(key) }} />

          <strong>{key}</strong>
        </Details>

        {value}
      </TooltipData>
    ));

    return (
      <Tooltip>
        <TooltipDate>
          {formatter(new Date(props.data[graphConfig.indexByColumn]))}
        </TooltipDate>

        {items}
      </Tooltip>
    );
  };
}
