import { formatRelative } from 'date-fns';
import { useHistory, useParams } from 'react-router';
import styled from 'styled-components';

import { H1 } from 'design';
import { Refresh } from 'design/Icon';

import { Days, Timeframe } from 'e-teleport/AccessMonitoring/Timeframe';
import cfg from 'e-teleport/config';

interface HeaderProps {
  disabled: boolean;
  title: string;
  onRefresh: () => void;
  lastUpdated: Date | null;
  days: Days;
}

const Container = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

const Controls = styled.div<{ disabled?: boolean }>`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: ${p => p.theme.space[3]}px;
  opacity: ${p => (p.disabled ? 0.5 : 1)};
  pointer-events: ${p => (p.disabled ? 'none' : 'all')};
`;

const RefreshButton = styled.div`
  border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  border-radius: 7px;
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 95px;
  box-sizing: border-box;
  cursor: pointer;
  gap: ${p => p.theme.space[1]}px;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

export function Header(props: HeaderProps) {
  const { name } = useParams<{
    name: string;
  }>();

  const history = useHistory();

  function handleChange(days: number) {
    history.replace(cfg.getAccessMonitoringReportRoute(name, days));
  }

  return (
    <Container>
      <H1>{props.title}</H1>

      <Controls disabled={props.disabled}>
        {props.lastUpdated && (
          <div>
            Last updated{' '}
            <strong>{formatRelative(props.lastUpdated, new Date())}</strong>
          </div>
        )}

        <RefreshButton onClick={props.onRefresh}>
          <Refresh size="medium" />
          Refresh
        </RefreshButton>

        <Timeframe days={props.days} onChange={handleChange} />
      </Controls>
    </Container>
  );
}
