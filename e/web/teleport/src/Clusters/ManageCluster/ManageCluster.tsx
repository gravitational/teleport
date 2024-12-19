import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router';
import styled from 'styled-components';

import { useAsync } from 'shared/hooks/useAsync';
import { Attempt } from 'shared/hooks/useAttemptNext';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { Clock, Edit } from 'design/Icon';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import Text, { H2 } from 'design/Text';
import { Indicator } from 'design/Indicator';

import {
  ClusterInformation,
  DataItem,
  IconBox,
  ManageClusterHeader,
} from 'teleport/Clusters/ManageCluster/ManageCluster';
import { FeatureBox } from 'teleport/components/Layout';
import { useNoMinWidth } from 'teleport/Main';
import cfg from 'teleport/config';
import { ClusterInfo } from 'teleport/services/clusters';

import { Alert } from 'design/Alert';

import { UpgradeWindowStartHour } from 'e-teleport/services/upgradeWindow';
import useTeleportE from 'e-teleport/useTeleportE';

import { Contacts } from './Contacts';
import { useUpgradeWindowStart } from './useUpgradeWindowStart';
import { ScheduleUpgrades } from './ScheduleUpgrades/ScheduleUpgrades';

export function ManageCluster() {
  const [cluster, setCluster] = useState<ClusterInfo>(null);
  const ctx = useTeleportE();

  const { clusterId } = useParams<{
    clusterId: string;
  }>();

  const [clusterAttempt, clusterRun] = useAsync(
    useCallback(async () => {
      const res = await ctx.clusterService.fetchClusterDetails(clusterId);
      setCluster(res);
      return res;
    }, [clusterId, ctx.clusterService])
  );

  const {
    fetchWindowAttempt,
    updateWindowAttempt,
    closeScheduleUpgrade,
    onUpdate,
    scheduleUpgradesVisible,
    selectedUpgradeWindowStart,
    setSelectedUpgradeWindowStart,
    showScheduleUpgrade,
  } = useUpgradeWindowStart(ctx, clusterId, cluster?.isCloud);

  useEffect(() => {
    if (!clusterAttempt.status && clusterId) {
      clusterRun();
    }
  }, [clusterAttempt.status, clusterRun, clusterId]);

  useNoMinWidth();

  const showUpgradeWindow = cluster?.isCloud;
  const showContacts = cfg.isDashboard || cluster?.isCloud;

  return (
    <FeatureBox>
      <ManageClusterHeader clusterId={clusterId} />
      {clusterAttempt.status === 'processing' ? (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : (
        <>
          <FlexRow gap="3">
            <ClusterInformation
              cluster={cluster}
              style={{ flexBasis: '100%' }}
              maxWidth={showUpgradeWindow ? '500px' : '100%'}
              attempt={clusterAttempt}
            />
            {showUpgradeWindow && (
              <ScheduledUpgrades
                showScheduleUpgrade={scheduleUpgradesVisible}
                selectedUpgradeWindowStart={selectedUpgradeWindowStart}
                onShow={showScheduleUpgrade}
                onUpdate={onUpdate}
                fetchWindowAttempt={fetchWindowAttempt}
                updateWindowAttempt={updateWindowAttempt}
                onClose={closeScheduleUpgrade}
                onSelectedWindowChange={setSelectedUpgradeWindowStart}
                selectedWindow={selectedUpgradeWindowStart}
              />
            )}
          </FlexRow>
          {showContacts && <Contacts />}
        </>
      )}
    </FeatureBox>
  );
}

const FlexRow = styled(Flex)`
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    flex-direction: column;
  }
`;

type ScheduledUpgradesProps = {
  selectedUpgradeWindowStart: UpgradeWindowStartHour;
  showScheduleUpgrade: boolean;
  onShow: () => void;
  onUpdate: () => Promise<boolean>;
  onClose: () => void;
  selectedWindow: UpgradeWindowStartHour;
  onSelectedWindowChange: (w: UpgradeWindowStartHour) => void;
  fetchWindowAttempt: Attempt;
  updateWindowAttempt: Attempt;
};

function ScheduledUpgrades({
  selectedUpgradeWindowStart,
  showScheduleUpgrade,
  onShow,
  onUpdate,
  onClose,
  selectedWindow,
  onSelectedWindowChange,
  fetchWindowAttempt,
  updateWindowAttempt,
}: ScheduledUpgradesProps) {
  return (
    <>
      <MultiRowBox mb={3} width="100%">
        <Row>
          <Flex alignItems="center" justifyContent="start">
            <IconBox>
              <Clock />
            </IconBox>
            <H2>Scheduled Upgrades</H2>
          </Flex>
        </Row>
        <Row>
          {fetchWindowAttempt.status === 'failed' && (
            <Alert>{fetchWindowAttempt.statusText}</Alert>
          )}
          {fetchWindowAttempt.status !== 'failed' && (
            <>
              <DataItem
                title="Window Start Time"
                data={
                  <Flex alignItems="center">
                    {makeLabel(selectedUpgradeWindowStart)}
                    <EditLink onClick={onShow} ml="2" size="medium" />
                  </Flex>
                }
                isLoading={fetchWindowAttempt.status === 'processing'}
              />
              <Text
                typography="body2"
                css={`
                  @media screen and (max-width: ${props =>
                      props.theme.breakpoints.mobile}px) {
                    margin-left: ${props => props.theme.space[2]}px;
                  }
                `}
              >
                Window Start Time is the hour in which an upgrade may begin.
                Changing this value changes it for everyone in your
                organization.
              </Text>
            </>
          )}
        </Row>
      </MultiRowBox>
      {showScheduleUpgrade && (
        <ScheduleUpgrades
          onSave={onUpdate}
          onCancel={onClose}
          selectedWindow={selectedWindow}
          onSelectedWindowChange={onSelectedWindowChange}
          attempt={updateWindowAttempt}
        />
      )}
    </>
  );
}

const makeLabel = (startHour: number): string => {
  return `${String(startHour).padStart(2, '0')}:00 (UTC)`;
};

const EditLink = styled(Edit)`
  color: ${props => props.theme.colors.text.slightlyMuted};
  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
    cursor: pointer;
  }
`;
