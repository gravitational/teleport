import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { useAttemptNext } from 'shared/hooks';

import useStickyClusterId from 'teleport/useStickyClusterId';

import { useParams } from 'react-router';

import Indicator from 'design/Indicator';

import { REPORT_VIEW_CONFIGS } from 'e-teleport/AccessMonitoring/Report/config';

import { Header } from 'e-teleport/AccessMonitoring/Report/Header';

import {
  getReport,
  getReportState,
  runReport,
} from 'e-teleport/AccessMonitoring/service';

import { Days } from 'e-teleport/AccessMonitoring/Timeframe';

import { ReportContent } from 'e-teleport/AccessMonitoring/Report/ReportContent';

import { ReportState } from '../types';

import type { Report } from '../types';

const Container = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[5]}px;
`;

const LoadingContainer = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  padding: ${p => p.theme.space[5]}px 0;
  flex-direction: column;
`;

export function Report() {
  const { clusterId } = useStickyClusterId();
  const { attempt, run } = useAttemptNext('processing');

  const { days, name } = useParams<{
    days: string;
    name: string;
  }>();

  const daysInt = parseInt(days, 10) as Days;

  const [reportStatus, setReportStatus] = useState<ReportState | null>(null);
  const [data, setData] = useState<Report | null>(null);
  const [shouldPoll, setShouldPoll] = useState(false);

  useEffect(() => {
    async function init() {
      if (isNaN(daysInt)) {
        return;
      }

      const state = await getReportState(clusterId, name, daysInt);

      if (
        state.status === ReportState.Ready ||
        state.status === ReportState.Running
      ) {
        const res = await getReport(clusterId, name, daysInt);

        setData(res);

        // even if it's running, we can grab the last report and display it, so we mark the status as ready
        setReportStatus(ReportState.Ready);
      }
    }

    run(init);
  }, [name, days]);

  useEffect(() => {
    if (!shouldPoll) {
      return;
    }

    const interval = setInterval(async () => {
      const state = await getReportState(clusterId, name, daysInt);

      setReportStatus(state.status);

      if (state.status === ReportState.Ready) {
        setShouldPoll(false);

        run(async () => {
          const res = await getReport(clusterId, name, daysInt);

          setData(res);
        });
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [shouldPoll]);

  function handleRefresh() {
    setShouldPoll(true);
    setReportStatus(ReportState.Running);

    run(() => runReport(clusterId, name, days));
  }

  const config = REPORT_VIEW_CONFIGS.get(name);

  let controlsDisabled = false;
  let error: JSX.Element | string | null = null;

  if (config === undefined) {
    error = 'The report does not exist.';
  }

  if (isNaN(daysInt)) {
    error = 'Invalid number of days. Select an option from the dropdown.';
  }

  if (attempt.status === 'processing') {
    error = (
      <LoadingContainer>
        <Indicator />
      </LoadingContainer>
    );

    controlsDisabled = true;
  }

  if (reportStatus === ReportState.Running) {
    error = (
      <LoadingContainer>
        <Indicator />
        The report is currently running. This may take a few minutes.
      </LoadingContainer>
    );

    controlsDisabled = true;
  }

  if (attempt.status === 'failed') {
    error =
      'There was an error loading the report. Try running the report again.';
  }

  if (reportStatus === ReportState.Failed) {
    error = 'The report failed to generate. Try running the report again.';
  }

  if (error) {
    return (
      <Container>
        <Header
          title={config.name}
          disabled={controlsDisabled}
          lastUpdated={null}
          onRefresh={handleRefresh}
          days={isNaN(daysInt) ? null : daysInt}
        />

        {error}
      </Container>
    );
  }

  return (
    <Container>
      <Header
        title={config.name}
        disabled={false}
        lastUpdated={data?.lastUpdated}
        onRefresh={handleRefresh}
        days={daysInt}
      />

      <ReportContent config={config} data={data} days={daysInt} />
    </Container>
  );
}
