import React, { useState } from 'react';
import styled from 'styled-components';

import { InfoFilled } from 'design/Icon/Icons/InfoFilled';

import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { H2 } from 'design';

import {
  GraphType,
  ReportGraphConfig,
  ReportViewConfig,
} from 'e-teleport/AccessMonitoring/Report/config';
import { QueryInfo } from 'e-teleport/AccessMonitoring/Report/QueryInfo';
import { Report, ReportResult } from 'e-teleport/AccessMonitoring/types';
import { BarGraph } from 'e-teleport/AccessMonitoring/Report/BarGraph/BarGraph';
import { Popover } from 'e-teleport/AccessMonitoring/shared/Popover';

interface ReportContentProps {
  config: ReportViewConfig;
  data: Report;
  days: number;
}

const Container = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
`;

const GraphContainer = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  border: 2px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 14px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[2]}px 0;
`;

const GraphHeader = styled.div`
  margin-bottom: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

const InfoPopover = styled(Popover)`
  top: 40px;
  width: 400px;
  right: -11px;

  &:before {
    top: -10px;
    right: 16px;
    border-width: 0 10px 10px 10px;
    border-color: transparent transparent
      ${p => p.theme.colors.spotBackground[0]} transparent;
  }

  &:after {
    top: -8px;
    right: 18px;
    border-width: 0 8px 8px 8px;
    border-color: transparent transparent ${p => p.theme.colors.levels.popout}
      transparent;
  }
`;

const InfoButton = styled.div`
  cursor: pointer;
  border-radius: 4px;
  display: flex;
  line-height: 1;
  align-items: center;
  justify-content: center;
  padding: 4px;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const Info = styled.div`
  position: relative;
`;

const GraphDetails = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[1]}px;
`;

const GraphDescription = styled.div`
  font-size: 14px;
`;

export function ReportContent(props: ReportContentProps) {
  const [openInfo, setOpenInfo] = useState(-1);

  const infoRef = useRefClickOutside<HTMLDivElement>({
    open: openInfo !== -1,
    setOpen: () => setOpenInfo(-1),
  });

  function handleInfoClick(event: React.MouseEvent, index: number) {
    event.preventDefault();
    event.stopPropagation();

    if (openInfo === index) {
      setOpenInfo(-1);
    } else {
      setOpenInfo(index);
    }
  }

  const graphs = props.config.graphs.map((graph, index) => {
    const graphData = props.data.results.find(
      result => result.name === graph.name
    );

    return (
      <GraphContainer key={index}>
        <GraphHeader>
          <GraphDetails>
            <H2>{graphData.title}</H2>
            <GraphDescription>{graphData.description}</GraphDescription>
          </GraphDetails>

          <Info>
            <InfoButton onClick={e => handleInfoClick(e, index)}>
              <InfoFilled />
            </InfoButton>

            {openInfo === index && (
              <InfoPopover ref={infoRef}>
                <QueryInfo query={graphData.query} days={props.days} />
              </InfoPopover>
            )}
          </Info>
        </GraphHeader>

        {getGraphComponent(graph, graphData, props.days)}
      </GraphContainer>
    );
  });

  return <Container>{graphs}</Container>;
}

function getGraphComponent(
  graphConfig: ReportGraphConfig,
  graphData: ReportResult,
  days: number
) {
  switch (graphConfig.type) {
    case GraphType.Bar:
      return (
        <BarGraph
          reportResult={graphData}
          graphConfig={graphConfig}
          days={days}
        />
      );
  }
}
