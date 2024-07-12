import React, { useEffect, useState } from 'react';
import styled, { css } from 'styled-components';
import { Lock } from 'design/Icon';
import { Link } from 'react-router-dom';
import { useAttemptNext } from 'shared/hooks';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { ShimmerBox } from 'design/ShimmerBox';

import { Alert } from 'design';

import { getReports } from 'e-teleport/AccessMonitoring/service';

import cfg from 'e-teleport/config';
import { DEFAULT_TIMEFRAME } from 'e-teleport/AccessMonitoring/const';

import { REPORT_VIEW_CONFIGS } from 'e-teleport/AccessMonitoring/Report/config';

import { ReportOverview } from './types';

const Container = styled.div<{ isLoading?: boolean }>`
  display: flex;
  flex-direction: column;
  pointer-events: ${p => (p.isLoading ? 'none' : 'auto')};
`;

const List = styled.div`
  padding: 0;
  display: flex;
  gap: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
`;

const itemStyles = css<{ disabled?: boolean }>`
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 7px;
  padding: ${p => p.theme.space[1] + p.theme.space[2]}px
    ${p => p.theme.space[3]}px;
  cursor: pointer;
  pointer-events: ${p => (p.disabled ? 'none' : 'auto')};
  opacity: ${p => (p.disabled ? 0.5 : 1)};
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[1]}px;
  color: ${p => p.theme.colors.text.main};
  text-decoration: none;

  &:hover {
    background: ${p => p.theme.colors.levels.surface};
  }
`;

const LinkItem = styled(Link)<{ disabled?: boolean }>`
  ${itemStyles};
`;

const Item = styled.div<{ disabled?: boolean }>`
  ${itemStyles};

  user-select: none;
  pointer-events: none;
`;

const Name = styled.h4`
  padding: 0;
  margin: 0;
  font-size: 18px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 25px;
`;

const Description = styled.div`
  margin: 0;
  width: 300px;
`;

export function ReportList() {
  const { clusterId } = useStickyClusterId();

  const { attempt, run } = useAttemptNext('processing');

  const [reports, setReports] = useState<ReportOverview[]>([]);

  useEffect(() => {
    async function init() {
      const reports = await getReports(clusterId);

      setReports(reports);
    }

    run(init);
  }, []);

  if (attempt.status === 'processing') {
    return <Loading />;
  }

  if (attempt.status === 'failed') {
    return <Alert>Failed to load the report list.</Alert>;
  }

  const items = reports.map((report, index) => (
    <LinkItem
      key={index}
      to={cfg.getAccessMonitoringReportRoute(report.name, DEFAULT_TIMEFRAME)}
    >
      <Name>{REPORT_VIEW_CONFIGS.get(report.name).name}</Name>

      <Description>
        {REPORT_VIEW_CONFIGS.get(report.name).description}
      </Description>
    </LinkItem>
  ));

  return (
    <Container>
      <h3>Built-in Reports</h3>

      <List>{items}</List>

      <h3>Coming Soon</h3>

      <List>
        <Item disabled>
          <Name>
            SOC 2 Report
            <div>
              <Lock size="medium" />
            </div>
          </Name>

          <Description>
            Review access control, exceptions and permissions
          </Description>
        </Item>

        <Item disabled>
          <Name>
            Unused Permissions Report
            <div>
              <Lock size="medium" />
            </div>
          </Name>

          <Description>
            Identify all permissions that are allocated but not actively used
          </Description>
        </Item>
      </List>
    </Container>
  );
}

function Loading() {
  return (
    <Container isLoading>
      <h3>Built-in Reports</h3>

      <List>
        <Item>
          <Name>
            <ShimmerBox width="200px" height="18px" />
          </Name>

          <Description>
            <ShimmerBox width="250px" height="12px" mt={1} />
            <ShimmerBox width="170px" height="12px" mt={1} />
            <ShimmerBox width="120px" height="12px" mt={1} />
          </Description>
        </Item>
      </List>

      <h3>Coming Soon</h3>

      <List>
        <Item>
          <Name>
            <ShimmerBox width="200px" height="18px" />
          </Name>

          <Description>
            <ShimmerBox width="250px" height="12px" mt={1} />
            <ShimmerBox width="170px" height="12px" mt={1} />
            <ShimmerBox width="120px" height="12px" mt={1} />
          </Description>
        </Item>

        <Item>
          <Name>
            <ShimmerBox width="200px" height="18px" />
          </Name>

          <Description>
            <ShimmerBox width="250px" height="12px" mt={1} />
            <ShimmerBox width="170px" height="12px" mt={1} />
            <ShimmerBox width="120px" height="12px" mt={1} />
          </Description>
        </Item>
      </List>
    </Container>
  );
}
