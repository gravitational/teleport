import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Edit } from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';

import type { UpgradeWindowStartHour } from 'e-teleport/services/upgradeWindow';
import {
  makeLabel,
  ScheduleUpgrades,
} from 'e-teleport/Support/ScheduleUpgrades';
import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';
import {
  Card,
  CardTitle,
  DocsLink,
  InfoItem,
  ManagedUpdates as ManagedUpdatesOSS,
} from 'teleport/ManagedUpdates';
import { ClusterMaintenanceInfo } from 'teleport/services/managedUpdates';

export default function ManagedUpdates() {
  if (!cfg.isCloud) {
    return <ManagedUpdatesOSS />;
  }

  return <ManagedUpdatesOSS ClusterMaintenanceCard={ClusterMaintenanceCard} />;
}

const DOCS_URL =
  'https://goteleport.com/docs/upgrading/cloud-cluster-updates/#maintenance-windows';

function ClusterMaintenanceCard({ data }: { data: ClusterMaintenanceInfo }) {
  const ctx = useTeleportE();
  const clusterId = ctx.storeUser.getClusterId();
  const { attempt: updateWindowAttempt, run: updateWindowRun } = useAttempt();

  const [scheduleUpgradesVisible, setScheduleUpgradesVisible] = useState(false);
  const [selectedUpgradeWindowStart, setSelectedUpgradeWindowStart] =
    useState<UpgradeWindowStartHour>(
      data.maintenanceStartHour as UpgradeWindowStartHour
    );

  useEffect(() => {
    setSelectedUpgradeWindowStart(
      data.maintenanceStartHour as UpgradeWindowStartHour
    );
  }, [data.maintenanceStartHour]);

  function showScheduleUpgrade() {
    setScheduleUpgradesVisible(true);
  }

  function closeScheduleUpgrade() {
    setScheduleUpgradesVisible(false);
  }

  function onUpdate() {
    return updateWindowRun(() =>
      ctx.upgradeWindowService
        .updateUpgradeWindowStart(clusterId, selectedUpgradeWindowStart)
        .then(closeScheduleUpgrade)
    );
  }

  return (
    <>
      <Card flex="1 1 50%">
        <CardTitle>Cluster Maintenance</CardTitle>
        <Flex alignItems="flex-start" gap={1} mb={3} flexDirection="column">
          <Text color="text.slightlyMuted">
            Auth and Proxy Service automatic updates will occur during the
            selected maintenance window below.
          </Text>
          <DocsLink docsUrl={DOCS_URL} />
        </Flex>
        <Box>
          <InfoItem
            label="Control Plane Version"
            value={data.controlPlaneVersion}
          />
          <InfoItem
            label="Maintenance Window"
            value={
              <Flex alignItems="center">
                {makeLabel(selectedUpgradeWindowStart)}
                <EditLink onClick={showScheduleUpgrade} ml={2} size="medium" />
              </Flex>
            }
          />
        </Box>
      </Card>
      {scheduleUpgradesVisible && (
        <ScheduleUpgrades
          onSave={onUpdate}
          onCancel={closeScheduleUpgrade}
          selectedWindow={selectedUpgradeWindowStart}
          onSelectedWindowChange={setSelectedUpgradeWindowStart}
          attempt={updateWindowAttempt}
        />
      )}
    </>
  );
}

const EditLink = styled(Edit)`
  color: ${props => props.theme.colors.text.slightlyMuted};
  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
    cursor: pointer;
  }
`;
